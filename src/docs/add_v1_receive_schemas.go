//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/bbernhard/signal-cli-rest-api/docs"
)

const (
	goDocsPath              = "docs.go"
	jsonDocsPath            = "swagger.json"
	openMarker              = "const docTemplate = `"
	closeMarker             = "`\n\n// SwaggerInfo"
	schemesTemplateValue    = "{{ marshal .Schemes }}"
	schemesPlaceholderToken = "__SWAG_SCHEMES_PLACEHOLDER__"
	receivePrefix           = "receive."
	receivePathKey          = "/v1/receive/{number}"
	receiveWrapper          = "data.Message"
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

	updateReceiveSchemaRefs(definitions, titleByFile)

	addEnvelopeWrapperDefinition(definitions)

	if err := updateDocsGo(definitions); err != nil {
		return err
	}

	if err := updateSwaggerJSON(definitions); err != nil {
		return err
	}

	fmt.Printf("updated %s\n", goDocsPath)
	fmt.Printf("updated %s\n", jsonDocsPath)
	return nil
}

func updateDocsGo(receiveDefinitions map[string]interface{}) error {
	content, err := os.ReadFile(goDocsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", goDocsPath, err)
	}

	template, templateStart, templateEnd, err := extractDocTemplate(string(content))
	if err != nil {
		return err
	}

	updatedTemplate, err := updateJSONDocument(toValidJson(template), receiveDefinitions)
	if err != nil {
		return err
	}
	updatedTemplate = encodeForGoRawString(updatedTemplate)

	updated := string(content[:templateStart]) + updatedTemplate + string(content[templateEnd:])
	if err := os.WriteFile(goDocsPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", goDocsPath, err)
	}

	return nil
}

func updateSwaggerJSON(receiveDefinitions map[string]interface{}) error {
	content, err := os.ReadFile(jsonDocsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", jsonDocsPath, err)
	}

	updated, err := updateJSONDocument(string(content), receiveDefinitions)
	if err != nil {
		return err
	}

	if err := os.WriteFile(jsonDocsPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", jsonDocsPath, err)
	}

	return nil
}

func extractDocTemplate(content string) (string, int, int, error) {
	start := strings.Index(content, openMarker)
	if start == -1 {
		return "", -1, -1, fmt.Errorf("could not find docTemplate start in %s", goDocsPath)
	}

	start += len(openMarker)
	endOffset := strings.Index(content[start:], closeMarker)
	if endOffset == -1 {
		return "", -1, -1, fmt.Errorf("could not find docTemplate end in %s", goDocsPath)
	}

	end := start + endOffset
	return content[start:end], start, end, nil
}

func toValidJson(content string) string {
	content = strings.ReplaceAll(content, "` + \"`\" + `", "`")
	content = strings.Replace(content, schemesTemplateValue, `"`+schemesPlaceholderToken+`"`, 1)
	return content
}

func encodeForGoRawString(content string) string {
	content = strings.ReplaceAll(content, "`", "` + \"`\" + `")
	content = strings.Replace(content, `"`+schemesPlaceholderToken+`"`, schemesTemplateValue, 1)
	return content
}

func updateJSONDocument(content string, receiveDefinitions map[string]interface{}) (string, error) {
	var document map[string]interface{}
	if err := json.Unmarshal([]byte(content), &document); err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}

	if err := applyReceiveSchemaUpdates(document, receiveDefinitions); err != nil {
		return "", err
	}

	raw, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal document: %w", err)
	}

	return string(raw), nil
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

func updateReceiveSchemaRefs(definitions map[string]interface{}, titleByFile map[string]string) {
	for key, value := range definitions {
		if !strings.HasPrefix(key, receivePrefix) {
			continue
		}
		definitions[key] = rewriteSchemaRefs(value, titleByFile)
	}
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

func applyReceiveSchemaUpdates(document map[string]interface{}, receiveDefinitions map[string]interface{}) error {
	definitions, err := getObject(document, "definitions")
	if err != nil {
		return err
	}

	for key := range definitions {
		if strings.HasPrefix(key, receivePrefix) || key == receiveWrapper {
			delete(definitions, key)
		}
	}

	for key, value := range receiveDefinitions {
		definitions[key] = value
	}

	paths, err := getObject(document, "paths")
	if err != nil {
		return err
	}
	receivePath, err := getObject(paths, receivePathKey)
	if err != nil {
		return err
	}
	receiveGet, err := getObject(receivePath, "get")
	if err != nil {
		return err
	}
	responses, err := getObject(receiveGet, "responses")
	if err != nil {
		return err
	}
	response200, err := getObject(responses, "200")
	if err != nil {
		return err
	}

	response200["schema"] = map[string]interface{}{
		"$ref": "#/definitions/" + receiveWrapper,
	}

	return nil
}

func getObject(parent map[string]interface{}, key string) (map[string]interface{}, error) {
	value, ok := parent[key]
	if !ok {
		return nil, fmt.Errorf("missing key %q", key)
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q is not an object", key)
	}

	return obj, nil
}
