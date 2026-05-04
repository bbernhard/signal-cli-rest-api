//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/bbernhard/signal-cli-rest-api/docs"
)

const (
	goDocsPath       = "docs.go"
	jsonDocsPath    = "swagger.json"
	openMarker     = "const docTemplate = `"
	closeMarker    = "`\n\n// SwaggerInfo"
	definitionsKey = `"definitions": {`
	receivePrefix  = "receive."
	receivePathKey = `"/v1/receive/{number}":`
	receiveWrapper = "data.Message"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: go run update_receive_docs.go <receiveDir>\n")
		os.Exit(1)
	}
	receiveDir := os.Args[1]

	if err := run(receiveDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(receiveDir string) error {
	definitions := make(map[string]interface{})

	titleByFile, err := addReceiveSchemas(definitions, receiveDir)
	if err != nil {
		return err
	}

	if err := updateReceiveSchemaRefs(definitions, titleByFile); err != nil {
		return err
	}

	addEnvelopeWrapperDefinition(definitions)

	managedDefinitions, err := renderManagedDefinitions(definitions)
	if err != nil {
		return err
	}

	if err := updateDocsGo(managedDefinitions); err != nil {
		return err
	}

	if err := updateSwaggerJSON(managedDefinitions); err != nil {
		return err
	}

	fmt.Printf("updated %s\n", goDocsPath)
	fmt.Printf("updated %s\n", jsonDocsPath)
	return nil
}

func updateDocsGo(managedDefinitions string) error {
	content, err := os.ReadFile(goDocsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", goDocsPath, err)
	}

	template, err := extractDocTemplate(string(content))
	if err != nil {
		return err
	}

	updatedTemplate, err := applyReceiveSchemaUpdates(template, managedDefinitions)
	if err != nil {
		return err
	}

	updated := strings.Replace(string(content), template, updatedTemplate, 1)
	if err := os.WriteFile(goDocsPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", goDocsPath, err)
	}

	return nil
}

func updateSwaggerJSON(managedDefinitions string) error {
	content, err := os.ReadFile(jsonDocsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", jsonDocsPath, err)
	}

	updated, err := applyReceiveSchemaUpdates(string(content), managedDefinitions)
	if err != nil {
		return err
	}

	if err := os.WriteFile(jsonDocsPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", jsonDocsPath, err)
	}

	return nil
}

func applyReceiveSchemaUpdates(content string, managedDefinitions string) (string, error) {
	updated, err := appendDefinitionsEntries(content, managedDefinitions)
	if err != nil {
		return "", err
	}

	updated, err = replaceReceiveResponseSchema(updated)
	if err != nil {
		return "", err
	}

	return updated, nil
}

func extractDocTemplate(content string) (string, error) {
	start := strings.Index(content, openMarker)
	if start == -1 {
		return "", fmt.Errorf("could not find docTemplate start in %s", goDocsPath)
	}

	start += len(openMarker)
	endOffset := strings.Index(content[start:], closeMarker)
	if endOffset == -1 {
		return "", fmt.Errorf("could not find docTemplate end in %s", goDocsPath)
	}

	return content[start : start+endOffset], nil
}

func definitionsBounds(template string) (int, int, error) {
	definitionsIndex := strings.Index(template, definitionsKey)
	if definitionsIndex == -1 {
		return -1, -1, fmt.Errorf("could not find definitions block in docTemplate")
	}

	braceIndex := definitionsIndex + strings.Index(definitionsKey, "{")
	closingBraceIndex, err := findMatchingBrace(template, braceIndex)
	if err != nil {
		return -1, -1, err
	}

	return braceIndex, closingBraceIndex, nil
}

func appendDefinitionsEntries(template string, entries string) (string, error) {
	braceIndex, closingBraceIndex, err := definitionsBounds(template)
	if err != nil {
		return "", err
	}

	definitionsBlock := template[braceIndex : closingBraceIndex+1]
	if strings.Contains(definitionsBlock, `"`+receiveWrapper+`"`) || strings.Contains(definitionsBlock, `"`+receivePrefix) {
		return "", fmt.Errorf("definitions already contain receive entries; run swag init first")
	}

	inner := strings.TrimSpace(template[braceIndex+1 : closingBraceIndex])

	if inner == "" {
		return template[:braceIndex+1] + "\n" + entries + "\n" + template[closingBraceIndex:], nil
	}

	closingLine := "\n    }"
	insertIndex := strings.LastIndex(template[:closingBraceIndex+1], closingLine)
	if insertIndex == -1 {
		return "", fmt.Errorf("could not determine definitions closing line")
	}

	return template[:insertIndex] + ",\n" + entries + template[insertIndex:], nil
}

func addReceiveSchemas(definitions map[string]interface{}, receiveDir string) (map[string]string, error) {
	entries, err := os.ReadDir(receiveDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", receiveDir, err)
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	titleByFile := make(map[string]string, len(files))

	for _, name := range files {
		fullPath := filepath.Join(receiveDir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read schema file %s: %w", fullPath, err)
		}

		var schemaObj map[string]interface{}
		if err := json.Unmarshal(data, &schemaObj); err != nil {
			return nil, fmt.Errorf("parse schema file %s: %w", fullPath, err)
		}

		title, ok := schemaObj["title"].(string)
		if !ok || strings.TrimSpace(title) == "" {
			return nil, fmt.Errorf("schema file %s is missing title", fullPath)
		}

		titleByFile[name] = title
		definitions[receivePrefix+title] = removeSchemaKeysRecursive(schemaObj, "")
	}

	return titleByFile, nil
}

func removeSchemaKeysRecursive(value interface{}, parentKey string) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		updated := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			if key == "$schema" || key == "$id" {
				continue
			}
			if key == "title" && parentKey != "properties" {
				continue
			}
			updated[key] = removeSchemaKeysRecursive(item, key)
		}
		return updated
	case []interface{}:
		updated := make([]interface{}, len(typed))
		for idx, item := range typed {
			updated[idx] = removeSchemaKeysRecursive(item, parentKey)
		}
		return updated
	default:
		return value
	}
}

func updateReceiveSchemaRefs(definitions map[string]interface{}, titleByFile map[string]string) error {
	for key, value := range definitions {
		if !strings.HasPrefix(key, receivePrefix) {
			continue
		}
		definitions[key] = rewriteSchemaRefs(value, titleByFile)
	}

	return nil
}

func rewriteSchemaRefs(value interface{}, titleByFile map[string]string) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		updated := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			if k == "$ref" {
				if refValue, ok := v.(string); ok {
					if strings.HasSuffix(refValue, ".schema.json") {
						base := filepath.Base(refValue)
						if title, exists := titleByFile[base]; exists {
							updated[k] = "#/definitions/" + receivePrefix + title
							continue
						}
					}
				}
			}

			updated[k] = rewriteSchemaRefs(v, titleByFile)
		}
		return updated
	case []interface{}:
		updated := make([]interface{}, len(typed))
		for idx, item := range typed {
			updated[idx] = rewriteSchemaRefs(item, titleByFile)
		}
		return updated
	default:
		return value
	}
}

func addEnvelopeWrapperDefinition(definitions map[string]interface{}) {
	definitions[receiveWrapper] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account":  map[string]interface{}{"type": "string"},
			"envelope": map[string]interface{}{"$ref": "#/definitions/receive.MessageEnvelope"},
		},
		"required": []interface{}{"account", "envelope"},
	}
}

func renderManagedDefinitions(definitions map[string]interface{}) (string, error) {
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		raw, err := json.MarshalIndent(definitions[key], "", "    ")
		if err != nil {
			return "", fmt.Errorf("marshal definition %s: %w", key, err)
		}

		lines := strings.Split(string(raw), "\n")
		for idx := range lines {
			lines[idx] = "        " + lines[idx]
		}

		entry := "        " + strconv.Quote(key) + ": " + strings.TrimPrefix(lines[0], "        ")
		if len(lines) > 1 {
			entry += "\n" + strings.Join(lines[1:], "\n")
		}

		parts = append(parts, entry)
	}

	return strings.Join(parts, ",\n"), nil
}

func replaceReceiveResponseSchema(template string) (string, error) {
	pathIndex := strings.Index(template, receivePathKey)
	if pathIndex == -1 {
		return "", fmt.Errorf("could not find receive path block; run swag init first")
	}
	braceOffset := strings.Index(template[pathIndex+len(receivePathKey):], "{")
	if braceOffset == -1 {
		return "", fmt.Errorf("could not find opening brace for receive path block")
	}
	pathOpenBrace := pathIndex + len(receivePathKey) + braceOffset
	pathCloseBrace, err := findMatchingBrace(template, pathOpenBrace)
	if err != nil {
		return "", err
	}

	pathBlock := template[pathOpenBrace : pathCloseBrace+1]

	oldSchema := `"schema": {
                            "type": "array",
                            "items": {
                                "type": "string"
                            }
                        }`

	newSchema := `"schema": {
                            "$ref": "#/definitions/data.Message"
                        }`

	updatedPathBlock := strings.Replace(pathBlock, oldSchema, newSchema, 1)
	if updatedPathBlock == pathBlock {
		return "", fmt.Errorf("could not replace /v1/receive schema; ensure generated docs are freshly generated by swag")
	}

	return template[:pathOpenBrace] + updatedPathBlock + template[pathCloseBrace+1:], nil
}

func findMatchingBrace(input string, openBraceIndex int) (int, error) {
	depth := 0
	inString := false
	escaped := false

	for index := openBraceIndex; index < len(input); index++ {
		char := input[index]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}

		if char == '"' {
			inString = true
			continue
		}

		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}

	return -1, fmt.Errorf("could not find matching brace")
}
