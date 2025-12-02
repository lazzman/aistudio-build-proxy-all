package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// cleanParameters recursively removes unsupported fields from JSON Schema
// Based on Gemini API documentation, the following OpenAPI 3.0 schema attributes are NOT supported:
// - additionalProperties
// - default
// - optional
// - maximum
// - oneOf
// Returns: modified (bool), removedFields (map of field->value pairs that were removed)
func cleanParameters(params map[string]interface{}, context string) (bool, map[string]interface{}) {
	modified := false
	removedFields := make(map[string]interface{})

	// List of unsupported fields to remove
	unsupportedFields := []string{
		"additionalProperties",
		"default",
		"optional",
		"maximum",
		"oneOf",
	}

	// Remove all unsupported fields and track their values
	for _, field := range unsupportedFields {
		if value, ok := params[field]; ok {
			removedFields[field] = value
			delete(params, field)
			log.Printf("[SCHEMA CLEANUP] Removed '%s' field (value: %v) from %s", field, value, context)
			modified = true
		}
	}

	// Recursively clean nested objects in properties
	if properties, ok := params["properties"].(map[string]interface{}); ok {
		for propName, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				nestedModified, nestedRemoved := cleanParameters(propMap, context+".properties."+propName)
				if nestedModified {
					modified = true
					for k, v := range nestedRemoved {
						removedFields[context+".properties."+propName+"."+k] = v
					}
				}
			}
		}
	}

	// Recursively clean items in arrays
	if items, ok := params["items"].(map[string]interface{}); ok {
		nestedModified, nestedRemoved := cleanParameters(items, context+".items")
		if nestedModified {
			modified = true
			for k, v := range nestedRemoved {
				removedFields[context+".items."+k] = v
			}
		}
	}

	// Recursively clean nested objects in items.properties
	if items, ok := params["items"].(map[string]interface{}); ok {
		if properties, ok := items["properties"].(map[string]interface{}); ok {
			for propName, prop := range properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					nestedModified, nestedRemoved := cleanParameters(propMap, context+".items.properties."+propName)
					if nestedModified {
						modified = true
						for k, v := range nestedRemoved {
							removedFields[context+".items.properties."+propName+"."+k] = v
						}
					}
				}
			}
		}
	}

	return modified, removedFields
}

// fixSystemInstruction removes the incorrect "role" field from systemInstruction
// Roo/Cline sends systemInstruction with role:"user" which causes 400 errors
func fixSystemInstruction(bodyBytes []byte) []byte {
	var requestBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		return bodyBytes
	}

	modified := false

	// Check if systemInstruction exists and has a role field
	if sysInst, ok := requestBody["systemInstruction"].(map[string]interface{}); ok {
		if roleValue, hasRole := sysInst["role"]; hasRole {
			// Remove the role field as it's not supported in systemInstruction
			delete(sysInst, "role")
			logMsg := "[SYSTEM_INSTRUCTION_FIX] Removed invalid 'role' field from systemInstruction"
			log.Println(logMsg)
			addLog("WARN", logMsg, map[string]interface{}{
				"removed_field": "role",
				"removed_value": roleValue,
			})
			modified = true
		}
	}

	// Check if generationConfig has thinkingConfig with thinkingLevel instead of thinkingBudget
	if genConfig, ok := requestBody["generationConfig"].(map[string]interface{}); ok {
		if thinkingCfg, ok := genConfig["thinkingConfig"].(map[string]interface{}); ok {
			if level, hasLevel := thinkingCfg["thinkingLevel"].(string); hasLevel {
				// Convert thinkingLevel to thinkingBudget
				// Based on working example: high = 26240 tokens
				var budget int
				switch level {
				case "high":
					budget = 26240
				case "medium":
					budget = 13120
				case "low":
					budget = 6560
				default:
					budget = 26240 // default to high
				}

				thinkingCfg["thinkingBudget"] = budget
				delete(thinkingCfg, "thinkingLevel")
				logMsg := fmt.Sprintf("[THINKING_CONFIG_FIX] Converted thinkingLevel '%s' to thinkingBudget %d", level, budget)
				log.Println(logMsg)
				addLog("INFO", logMsg, map[string]interface{}{
					"original_field":  "thinkingLevel",
					"original_value":  level,
					"converted_field": "thinkingBudget",
					"converted_value": budget,
				})
				modified = true
			}
		}
	}

	if !modified {
		return bodyBytes
	}

	// Re-marshal the modified request body
	fixedBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshaling after fixes: %v", err)
		return bodyBytes
	}
	return fixedBody
}

// fixToolDefinitions transforms tool definitions from Roo/Cline format to Gemini API format
// Roo/Cline sends "parametersJsonSchema" but Gemini API expects "parameters"
// Also converts "functionDeclarations" (camelCase) to "function_declarations" (snake_case)
func fixToolDefinitions(bodyBytes []byte) []byte {
	var requestBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		// If we can't parse it, return original body
		return bodyBytes
	}

	// Check if request contains tools
	tools, ok := requestBody["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		// No tools, return original body
		return bodyBytes
	}

	// Track all transformations for logging
	transformations := make([]map[string]interface{}, 0)
	totalRemovedFields := make(map[string]interface{})
	toolCount := 0

	// Transform each tool's function declarations
	modified := false
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for both camelCase and snake_case versions
		// Gemini expects "function_declarations" (snake_case) but some clients send "functionDeclarations" (camelCase)
		var functionDeclarations []interface{}
		if funcDecls, ok := toolMap["functionDeclarations"].([]interface{}); ok {
			functionDeclarations = funcDecls
			// Rename to snake_case
			toolMap["function_declarations"] = funcDecls
			delete(toolMap, "functionDeclarations")
			modified = true
			log.Println("[TOOL FIX] Renamed functionDeclarations to function_declarations")
		} else if funcDecls, ok := toolMap["function_declarations"].([]interface{}); ok {
			functionDeclarations = funcDecls
		} else {
			continue
		}

		for _, funcDecl := range functionDeclarations {
			funcMap, ok := funcDecl.(map[string]interface{})
			if !ok {
				continue
			}

			toolName := "unknown"
			if name, ok := funcMap["name"].(string); ok {
				toolName = name
			}
			toolCount++

			toolTransform := map[string]interface{}{
				"tool_name": toolName,
				"changes":   make([]string, 0),
			}

			// Check if it has "parametersJsonSchema" instead of "parameters"
			if parametersJsonSchema, ok := funcMap["parametersJsonSchema"]; ok {
				// Rename it to "parameters"
				funcMap["parameters"] = parametersJsonSchema
				delete(funcMap, "parametersJsonSchema")
				modified = true
				logMsg := fmt.Sprintf("[TOOL FIX] Renamed parametersJsonSchema to parameters for function: %s", toolName)
				log.Println(logMsg)
				toolTransform["changes"] = append(toolTransform["changes"].([]string), "Renamed parametersJsonSchema -> parameters")
			}

			// Also check and fix camelCase vs snake_case for parameters field
			// Some clients might send parameters with camelCase keys that should be snake_case
			if params, ok := funcMap["parameters"].(map[string]interface{}); ok {
				// Check for common camelCase fields that should be snake_case
				if required, ok := params["required"].([]interface{}); ok {
					// Required is correct, no change needed
					_ = required
				}
			}

			// Clean up the parameters object - remove unsupported fields
			if parameters, ok := funcMap["parameters"].(map[string]interface{}); ok {
				wasModified, removedFields := cleanParameters(parameters, toolName)
				if wasModified {
					modified = true
					// Merge removed fields into total
					for k, v := range removedFields {
						fullKey := toolName + "." + k
						totalRemovedFields[fullKey] = v
					}
					toolTransform["removed_fields"] = removedFields
					toolTransform["removed_count"] = len(removedFields)
				}
			}

			if len(toolTransform["changes"].([]string)) > 0 || toolTransform["removed_fields"] != nil {
				transformations = append(transformations, toolTransform)
			}
		}
	}

	if !modified {
		// No changes needed, return original
		return bodyBytes
	}

	// Re-marshal the modified request body
	fixedBody, err := json.Marshal(requestBody)
	if err != nil {
		// If marshaling fails, return original body
		log.Printf("Error marshaling fixed tools: %v", err)
		return bodyBytes
	}

	// Log comprehensive transformation summary to web UI
	logMsg := fmt.Sprintf("[TOOL FIX] Transformed %d tool definitions for Gemini API compatibility", toolCount)
	log.Println(logMsg)
	addLog("INFO", logMsg, map[string]interface{}{
		"total_tools":               toolCount,
		"total_removed_fields":      len(totalRemovedFields),
		"transformations":           transformations,
		"all_removed_fields_detail": totalRemovedFields,
	})

	return fixedBody
}
