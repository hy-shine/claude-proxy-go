package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"

	"github.com/1rgs/claude-code-proxy-go/internal/types"
)

type ChatOptions struct {
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	TopK             *int
	Thinking         *types.ThinkingConfig
	Stop             []string
	Tools            []*schema.ToolInfo
	ToolChoice       *schema.ToolChoice
	AllowedToolNames []string
}

func ToEinoRequest(req *types.MessagesRequest) ([]*schema.Message, *ChatOptions, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("request is required")
	}
	if req.TopK != nil && *req.TopK < 0 {
		return nil, nil, fmt.Errorf("top_k must be >= 0")
	}
	if req.Thinking != nil {
		if req.Thinking.BudgetTokens < 0 {
			return nil, nil, fmt.Errorf("thinking.budget_tokens must be >= 0")
		}
		if req.Thinking.Enabled && req.Thinking.BudgetTokens <= 0 {
			return nil, nil, fmt.Errorf("thinking.budget_tokens must be > 0 when thinking.enabled is true")
		}
	}

	messages, err := convertMessages(req.Messages, req.System)
	if err != nil {
		return nil, nil, err
	}

	opts := &ChatOptions{
		Temperature: req.Temperature,
		MaxTokens:   intPtr(req.MaxTokens),
		TopP:        req.TopP,
		TopK:        req.TopK,
		Thinking:    req.Thinking,
		Stop:        req.StopSequences,
	}

	if len(req.Tools) > 0 {
		tools, err := convertTools(req.Tools)
		if err != nil {
			return nil, nil, err
		}
		opts.Tools = tools
	}

	if req.ToolChoice != nil {
		if len(req.Tools) == 0 {
			return nil, nil, fmt.Errorf("tool_choice requires tools")
		}
		choice, allowList, err := convertToolChoice(req.ToolChoice)
		if err != nil {
			return nil, nil, err
		}
		opts.ToolChoice = choice
		opts.AllowedToolNames = allowList
	}

	return messages, opts, nil
}

func intPtr(i int) *int {
	return &i
}

func convertMessages(msgs []types.Message, system interface{}) ([]*schema.Message, error) {
	var result []*schema.Message

	if system != nil {
		systemContent, err := extractSystemContent(system)
		if err != nil {
			return nil, err
		}
		if systemContent != "" {
			result = append(result, schema.SystemMessage(systemContent))
		}
	}

	for _, msg := range msgs {
		einoMsgs, err := convertMessage(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, einoMsgs...)
	}

	return result, nil
}

func extractSystemContent(system interface{}) (string, error) {
	switch v := system.(type) {
	case string:
		return v, nil
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if block["type"] == "text" {
					if text, ok := block["text"].(string); ok {
						sb.WriteString(text)
						sb.WriteString("\n\n")
					}
				}
			}
		}
		return strings.TrimSpace(sb.String()), nil
	default:
		return fmt.Sprintf("%v", system), nil
	}
}

func convertMessage(msg types.Message) ([]*schema.Message, error) {
	role := schema.User
	switch msg.Role {
	case "system":
		role = schema.System
	case "assistant":
		role = schema.Assistant
	case "tool":
		role = schema.Tool
	case "user":
		role = schema.User
	default:
		return nil, fmt.Errorf("unsupported role: %s", msg.Role)
	}

	switch v := msg.Content.(type) {
	case string:
		return []*schema.Message{{
			Role:    role,
			Content: v,
		}}, nil
	case []any:
		return convertBlockContent(role, v)
	default:
		return nil, fmt.Errorf("unsupported content format")
	}
}

func convertBlockContent(role schema.RoleType, blocks []any) ([]*schema.Message, error) {
	switch role {
	case schema.Assistant:
		return convertAssistantBlocks(blocks)
	case schema.User:
		return convertUserBlocks(blocks)
	case schema.Tool, schema.System:
		content, err := extractMessageContent(blocks)
		if err != nil {
			return nil, err
		}
		return []*schema.Message{{
			Role:    role,
			Content: content,
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported role for block conversion: %s", role)
	}
}

func extractMessageContent(content any) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []any:
		var sb strings.Builder
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				return "", fmt.Errorf("invalid content block format")
			}

			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				text, _ := block["text"].(string)
				sb.WriteString(text)
			case "tool_result":
				sb.WriteString(ParseToolResultContent(block["content"]))
			default:
				return "", fmt.Errorf("unsupported content block type: %s", blockType)
			}
		}
		return sb.String(), nil
	default:
		return "", fmt.Errorf("unsupported content format")
	}
}

func convertAssistantBlocks(blocks []any) ([]*schema.Message, error) {
	var (
		sb        strings.Builder
		toolCalls []schema.ToolCall
	)

	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid content block format")
		}

		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			text, _ := block["text"].(string)
			sb.WriteString(text)
		case "tool_use":
			toolID, _ := block["id"].(string)
			name, _ := block["name"].(string)
			args := toJSONString(block["input"])
			toolCalls = append(toolCalls, schema.ToolCall{
				ID:   toolID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      name,
					Arguments: args,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported content block type: %s", blockType)
		}
	}

	msg := &schema.Message{
		Role:      schema.Assistant,
		Content:   sb.String(),
		ToolCalls: toolCalls,
	}
	return []*schema.Message{msg}, nil
}

func convertUserBlocks(blocks []any) ([]*schema.Message, error) {
	var (
		result      []*schema.Message
		textBuilder strings.Builder
		parts       []schema.MessageInputPart
		multiMode   bool
	)

	flushTextToParts := func() {
		if textBuilder.Len() == 0 {
			return
		}
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: textBuilder.String(),
		})
		textBuilder.Reset()
	}

	flushUser := func() {
		if multiMode {
			flushTextToParts()
			if len(parts) == 0 {
				return
			}
			result = append(result, &schema.Message{
				Role:                  schema.User,
				UserInputMultiContent: parts,
			})
			parts = nil
			multiMode = false
			return
		}
		if textBuilder.Len() == 0 {
			return
		}
		result = append(result, schema.UserMessage(textBuilder.String()))
		textBuilder.Reset()
	}

	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid content block format")
		}

		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			text, _ := block["text"].(string)
			textBuilder.WriteString(text)
		case "image":
			imagePart, err := convertUserImageBlock(block)
			if err != nil {
				return nil, err
			}
			if !multiMode {
				flushTextToParts()
				multiMode = true
			} else {
				flushTextToParts()
			}
			parts = append(parts, imagePart)
		case "tool_result":
			flushUser()
			toolUseID, _ := block["tool_use_id"].(string)
			toolName, _ := block["name"].(string)
			content := ParseToolResultContent(block["content"])
			result = append(result, &schema.Message{
				Role:       schema.Tool,
				Content:    content,
				ToolCallID: toolUseID,
				ToolName:   toolName,
			})
		default:
			return nil, fmt.Errorf("unsupported content block type: %s", blockType)
		}
	}

	flushUser()
	if len(result) == 0 {
		result = append(result, schema.UserMessage(""))
	}

	return result, nil
}

func convertUserImageBlock(block map[string]any) (schema.MessageInputPart, error) {
	source, ok := block["source"].(map[string]any)
	if !ok {
		return schema.MessageInputPart{}, fmt.Errorf("image block source must be an object")
	}

	sourceType, _ := source["type"].(string)
	switch sourceType {
	case "base64":
		mediaType, _ := source["media_type"].(string)
		data, _ := source["data"].(string)
		if mediaType == "" {
			return schema.MessageInputPart{}, fmt.Errorf("image base64 source requires media_type")
		}
		if data == "" {
			return schema.MessageInputPart{}, fmt.Errorf("image base64 source requires data")
		}
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: strPtr(data),
					MIMEType:   mediaType,
				},
			},
		}, nil
	case "url":
		url, _ := source["url"].(string)
		if url == "" {
			return schema.MessageInputPart{}, fmt.Errorf("image url source requires url")
		}
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL: strPtr(url),
				},
			},
		}, nil
	default:
		return schema.MessageInputPart{}, fmt.Errorf("unsupported image source type: %s", sourceType)
	}
}

func strPtr(v string) *string {
	return &v
}

func toJSONString(v any) string {
	if v == nil {
		return "{}"
	}
	if s, ok := v.(string); ok {
		return s
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func convertTools(tools []types.Tool) ([]*schema.ToolInfo, error) {
	var result []*schema.ToolInfo

	for _, tool := range tools {
		var inputSchema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &inputSchema); err != nil {
			return nil, fmt.Errorf("invalid tool input_schema for %s: %w", tool.Name, err)
		}

		// Use simple parameter info map
		params := make(map[string]*schema.ParameterInfo)
		if props, ok := inputSchema["properties"].(map[string]any); ok {
			for name, prop := range props {
				if propMap, ok := prop.(map[string]any); ok {
					paramInfo := buildParameterInfo(propMap)
					paramInfo.Required = isRequired(inputSchema, name)
					params[name] = paramInfo
				}
			}
		}

		paramsOneOf := schema.NewParamsOneOfByParams(params)

		info := &schema.ToolInfo{
			Name:        tool.Name,
			Desc:        tool.Description,
			ParamsOneOf: paramsOneOf,
		}
		result = append(result, info)
	}

	return result, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func buildParameterInfo(node map[string]any) *schema.ParameterInfo {
	param := &schema.ParameterInfo{
		Type: parseDataType(node),
		Desc: getString(node, "description"),
	}

	if enumVals, ok := node["enum"].([]any); ok {
		for _, val := range enumVals {
			if s, ok := val.(string); ok {
				param.Enum = append(param.Enum, s)
			}
		}
	}

	if param.Type == schema.Array {
		if items, ok := node["items"].(map[string]any); ok {
			param.ElemInfo = buildParameterInfo(items)
		} else {
			param.ElemInfo = &schema.ParameterInfo{Type: schema.String}
		}
	}

	if param.Type == schema.Object {
		if props, ok := node["properties"].(map[string]any); ok {
			requiredSet := requiredNames(node)
			param.SubParams = make(map[string]*schema.ParameterInfo, len(props))
			for name, sub := range props {
				subMap, ok := sub.(map[string]any)
				if !ok {
					continue
				}
				subParam := buildParameterInfo(subMap)
				subParam.Required = requiredSet[name]
				param.SubParams[name] = subParam
			}
		}
	}

	return param
}

func parseDataType(node map[string]any) schema.DataType {
	if t, ok := node["type"].(string); ok {
		return toDataType(t)
	}

	// JSON schema may encode unions like ["string", "null"].
	if typeList, ok := node["type"].([]any); ok {
		for _, item := range typeList {
			if t, ok := item.(string); ok && t != "null" {
				return toDataType(t)
			}
		}
	}

	// Fallback by shape.
	if _, ok := node["properties"].(map[string]any); ok {
		return schema.Object
	}
	if _, ok := node["items"]; ok {
		return schema.Array
	}

	return schema.String
}

func toDataType(raw string) schema.DataType {
	switch raw {
	case "object":
		return schema.Object
	case "number":
		return schema.Number
	case "integer":
		return schema.Integer
	case "string":
		return schema.String
	case "array":
		return schema.Array
	case "null":
		return schema.Null
	case "boolean":
		return schema.Boolean
	default:
		return schema.String
	}
}

func requiredNames(node map[string]any) map[string]bool {
	out := make(map[string]bool)
	if required, ok := node["required"].([]any); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

func isRequired(schema map[string]any, name string) bool {
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			if r == name {
				return true
			}
		}
	}
	return false
}

func convertToolChoice(tc *types.ToolChoice) (*schema.ToolChoice, []string, error) {
	if tc == nil {
		return nil, nil, nil
	}

	switch tc.Type {
	case "auto":
		choice := schema.ToolChoiceAllowed
		return &choice, nil, nil
	case "any":
		choice := schema.ToolChoiceForced
		return &choice, nil, nil
	case "tool":
		if tc.Name == "" {
			return nil, nil, fmt.Errorf("tool_choice.name is required when type is tool")
		}
		choice := schema.ToolChoiceForced
		return &choice, []string{tc.Name}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported tool_choice.type: %s", tc.Type)
	}
}

func GetOpenAIConfig(apiKey, baseURL, model string) *openai.ChatModelConfig {
	cfg := openai.ChatModelConfig{
		Model:  model,
		APIKey: apiKey,
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &cfg
}
