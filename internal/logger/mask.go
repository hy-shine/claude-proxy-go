package logger

import (
	"encoding/json"
	"regexp"
)

var apiKeyPattern = regexp.MustCompile(`"apiKey"\s*:\s*"[^"]*"`)

// MaskAPIKeys masks all apiKey values in the given value by serializing to JSON
// and replacing apiKey values with "***".
func MaskAPIKeys(v any) any {
	if v == nil {
		return nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return v
	}

	masked := apiKeyPattern.ReplaceAll(data, []byte(`"apiKey":"***"`))

	var result any
	if err := json.Unmarshal(masked, &result); err != nil {
		return v
	}

	return result
}
