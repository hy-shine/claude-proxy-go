package converter

func CleanSchemaForGemini(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	// Remove unsupported fields
	delete(schema, "additionalProperties")
	delete(schema, "default")

	// Check for unsupported format in string types
	if typ, ok := schema["type"].(string); ok && typ == "string" {
		if format, ok := schema["format"].(string); ok {
			allowedFormats := map[string]bool{
				"enum":      true,
				"date-time": true,
			}
			if !allowedFormats[format] {
				delete(schema, "format")
			}
		}
	}

	// Recursively clean nested schemas
	for key, value := range schema {
		if nested, ok := value.(map[string]any); ok {
			schema[key] = CleanSchemaForGemini(nested)
		} else if arr, ok := value.([]any); ok {
			var cleanedArr []any
			for _, item := range arr {
				if itemMap, ok := item.(map[string]any); ok {
					cleanedArr = append(cleanedArr, CleanSchemaForGemini(itemMap))
				} else {
					cleanedArr = append(cleanedArr, item)
				}
			}
			schema[key] = cleanedArr
		}
	}

	return schema
}
