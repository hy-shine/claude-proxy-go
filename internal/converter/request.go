package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"

	"github.com/hy-shine/claude-proxy-go/internal/types"
)

type ChatOptions struct {
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	TopK             *int
	Thinking         *types.ThinkingConfig
	OutputConfig     *types.OutputConfig
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
	thinkingCfg, err := normalizeThinking(req.Thinking)
	if err != nil {
		return nil, nil, err
	}
	if thinkingCfg != nil {
		if thinkingCfg.BudgetTokens < 0 {
			return nil, nil, fmt.Errorf("thinking.budget_tokens must be >= 0")
		}
		if thinkingCfg.Type == "enabled" && thinkingCfg.BudgetTokens <= 0 {
			return nil, nil, fmt.Errorf("thinking.budget_tokens must be > 0 when thinking.type is enabled")
		}
	}

	outputCfg, err := normalizeOutputConfig(req.OutputConfig)
	if err != nil {
		return nil, nil, err
	}

	fallbackToolName := ""
	if len(req.Tools) == 1 {
		fallbackToolName = req.Tools[0].Name
	}

	messages, err := convertMessages(req.Messages, req.System, fallbackToolName)
	if err != nil {
		return nil, nil, err
	}

	opts := &ChatOptions{
		Temperature:  req.Temperature,
		MaxTokens:    intPtr(req.MaxTokens),
		TopP:         req.TopP,
		TopK:         req.TopK,
		Thinking:     thinkingCfg,
		OutputConfig: outputCfg,
		Stop:         req.StopSequences,
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

func convertMessages(msgs []types.Message, system any, fallbackToolName string) ([]*schema.Message, error) {
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
		einoMsgs, err := convertMessage(msg, fallbackToolName)
		if err != nil {
			return nil, err
		}
		result = append(result, einoMsgs...)
	}

	return result, nil
}

func extractSystemContent(system any) (string, error) {
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

func convertMessage(msg types.Message, fallbackToolName string) ([]*schema.Message, error) {
	var role schema.RoleType
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
		return convertBlockContent(role, v, fallbackToolName)
	default:
		return nil, fmt.Errorf("unsupported content format")
	}
}

func convertBlockContent(role schema.RoleType, blocks []any, fallbackToolName string) ([]*schema.Message, error) {
	switch role {
	case schema.Assistant:
		return convertAssistantBlocks(blocks, fallbackToolName)
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
			case "document":
				docText, err := convertDocumentBlock(block)
				if err != nil {
					return "", err
				}
				sb.WriteString(docText)
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

func convertAssistantBlocks(blocks []any, fallbackToolName string) ([]*schema.Message, error) {
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
		case "document":
			docText, err := convertDocumentBlock(block)
			if err != nil {
				return nil, err
			}
			sb.WriteString(docText)
		case "tool_use":
			toolID, _ := block["id"].(string)
			name, _ := block["name"].(string)
			if toolID == "" {
				return nil, fmt.Errorf("tool_use.id is required")
			}
			name = normalizeToolName(name, toolID, fallbackToolName)
			args := toJSONString(block["input"])
			toolCalls = append(toolCalls, schema.ToolCall{
				ID:   toolID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      name,
					Arguments: args,
				},
			})
		case "thinking", "redacted_thinking":
			// Skip thinking blocks — OpenAI compatible providers do not accept them.
			// The reasoning content will be reconstructed by the upstream model if needed.
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
		case "document":
			docText, err := convertDocumentBlock(block)
			if err != nil {
				return nil, err
			}
			textBuilder.WriteString(docText)
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

func convertDocumentBlock(block map[string]any) (string, error) {
	title := strings.TrimSpace(getString(block, "title"))
	context := strings.TrimSpace(getString(block, "context"))
	if text := strings.TrimSpace(getString(block, "text")); text != "" {
		return formatDocumentText(title, context, text), nil
	}

	source, ok := block["source"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("document block requires text or source object")
	}

	sourceType := strings.TrimSpace(getString(source, "type"))
	switch sourceType {
	case "text":
		text := strings.TrimSpace(getString(source, "text"))
		if text == "" {
			return "", fmt.Errorf("document text source requires text")
		}
		return formatDocumentText(title, context, text), nil
	case "url":
		url := strings.TrimSpace(getString(source, "url"))
		if url == "" {
			return "", fmt.Errorf("document url source requires url")
		}
		return formatDocumentReference(title, context, fmt.Sprintf("Document URL: %s", url)), nil
	case "base64":
		mediaType := strings.TrimSpace(getString(source, "media_type"))
		data := getString(source, "data")
		if mediaType == "" {
			return "", fmt.Errorf("document base64 source requires media_type")
		}
		if data == "" {
			return "", fmt.Errorf("document base64 source requires data")
		}
		return formatDocumentReference(
			title,
			context,
			fmt.Sprintf("Document attachment (media_type=%s, base64_bytes=%d)", mediaType, len(data)),
		), nil
	default:
		return "", fmt.Errorf("unsupported document source type: %s", sourceType)
	}
}

func formatDocumentText(title, context, text string) string {
	var sb strings.Builder
	if title != "" {
		sb.WriteString("Document title: ")
		sb.WriteString(title)
		sb.WriteString("\n")
	}
	if context != "" {
		sb.WriteString("Document context: ")
		sb.WriteString(context)
		sb.WriteString("\n")
	}
	sb.WriteString(text)
	return sb.String()
}

func formatDocumentReference(title, context, body string) string {
	var sb strings.Builder
	if title != "" {
		sb.WriteString("Document title: ")
		sb.WriteString(title)
		sb.WriteString("\n")
	}
	if context != "" {
		sb.WriteString("Document context: ")
		sb.WriteString(context)
		sb.WriteString("\n")
	}
	sb.WriteString(body)
	return sb.String()
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

func normalizeToolName(name, toolID, fallback string) string {
	if v := sanitizeToolName(name); v != "" {
		return v
	}
	if v := sanitizeToolName(fallback); v != "" {
		return v
	}
	if v := sanitizeToolName(toolID); v != "" {
		return v
	}
	return "tool"
}

func sanitizeToolName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(raw))
	lastUnderscore := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}

	out := strings.Trim(b.String(), "_-")
	if out == "" {
		return "tool"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func normalizeThinking(thinking *types.ThinkingConfig) (*types.ThinkingConfig, error) {
	if thinking == nil {
		return nil, nil
	}

	out := *thinking
	out.Type = strings.ToLower(strings.TrimSpace(out.Type))
	switch out.Type {
	case "", "enabled", "disabled", "adaptive":
	default:
		return nil, fmt.Errorf("thinking.type must be enabled, disabled, or adaptive, but got %s", out.Type)
	}

	if out.Display != "" {
		out.Display = strings.ToLower(strings.TrimSpace(out.Display))
		switch out.Display {
		case "", "summarized", "omitted":
		default:
			return nil, fmt.Errorf("thinking.display must be summarized or omitted, but got %s", out.Display)
		}
	}

	if out.Enabled && out.Type == "disabled" {
		return nil, fmt.Errorf("thinking.enabled conflicts with thinking.type=disabled")
	}

	switch out.Type {
	case "enabled":
		out.Enabled = true
	case "disabled":
		out.Enabled = false
	}

	return &out, nil
}

func normalizeOutputConfig(cfg *types.OutputConfig) (*types.OutputConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	out := *cfg
	if out.Effort != "" {
		out.Effort = strings.ToLower(strings.TrimSpace(out.Effort))
		switch out.Effort {
		case "", "low", "medium", "high", "max":
		default:
			return nil, fmt.Errorf("output_config.effort must be low, medium, high, or max, but got %s", out.Effort)
		}
	}

	return &out, nil
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
