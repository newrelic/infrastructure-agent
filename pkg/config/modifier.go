package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// UpdateConfigKey reads a YAML file, updates or adds a specific key, and saves it while preserving comments.
func UpdateConfigKey(filePath, targetKey, newValue string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml structure: expected mapping node")
	}

	content := root.Content[0].Content
	keyFound := false

	for i := 0; i < len(content); i += 2 {
		keyNode := content[i]
		valueNode := content[i+1]

		if keyNode.Value == targetKey {
			valueNode.Value = newValue
			valueNode.Kind = yaml.ScalarNode
			keyFound = true
			break
		}
	}

	if !keyFound {
		content = append(content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: targetKey},
			&yaml.Node{Kind: yaml.ScalarNode, Value: newValue},
		)
		root.Content[0].Content = content
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return fmt.Errorf("failed to encode yaml: %w", err)
	}

	return os.WriteFile(filePath, buf.Bytes(), 0644)
}
