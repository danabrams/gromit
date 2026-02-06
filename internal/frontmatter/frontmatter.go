package frontmatter

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse extracts YAML frontmatter and body from content.
// Returns the frontmatter map, body text, and any error.
// If no frontmatter is present, returns empty map and full content as body.
func Parse(content string) (map[string]interface{}, string, error) {
	// Check if content starts with ---
	if !strings.HasPrefix(content, "---\n") {
		// No frontmatter, return empty map and full content as body
		return make(map[string]interface{}), content, nil
	}

	// Find the closing ---
	rest := content[4:] // Skip opening "---\n"
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx == -1 {
		return nil, "", fmt.Errorf("unclosed frontmatter delimiter")
	}

	// Extract frontmatter YAML and body
	yamlContent := rest[:endIdx]
	body := rest[endIdx+5:] // Skip "\n---\n"

	// Parse YAML
	var frontmatter map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return nil, "", fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	return frontmatter, body, nil
}

// Serialize combines frontmatter and body into a single markdown string.
// If frontmatter is empty, returns just the body without delimiters.
func Serialize(frontmatter map[string]interface{}, body string) (string, error) {
	if len(frontmatter) == 0 {
		return body, nil
	}

	yamlBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("marshalling frontmatter: %w", err)
	}

	// Build the document with frontmatter
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(yamlBytes)
	sb.WriteString("---\n")
	sb.WriteString(body)

	return sb.String(), nil
}

// ReadFile reads a file and parses its frontmatter and body.
func ReadFile(path string) (map[string]interface{}, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading file: %w", err)
	}

	return Parse(string(data))
}

// UpdateFile reads a file, updates the frontmatter with the provided values, and writes it back.
// The updates map contains key-value pairs to set in the frontmatter.
// Existing keys not in updates are preserved.
func UpdateFile(path string, updates map[string]interface{}) error {
	// Read the file
	frontmatter, body, err := ReadFile(path)
	if err != nil {
		return err
	}

	// Apply updates
	if frontmatter == nil {
		frontmatter = make(map[string]interface{})
	}
	for k, v := range updates {
		frontmatter[k] = v
	}

	// Serialize and write back
	content, err := Serialize(frontmatter, body)
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0644)
}
