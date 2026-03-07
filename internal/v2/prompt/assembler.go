package prompt

import "strings"

// PromptAssembler merges prompt layers into a single payload.
type PromptAssembler struct {
	base     string
	project  string
	instance string
	fragment string
}

// NewPromptAssembler returns an assembler initialized with the provided layers.
func NewPromptAssembler(base, project, instance, fragment string) *PromptAssembler {
	return &PromptAssembler{base: base, project: project, instance: instance, fragment: fragment}
}

// Assemble concatenates the configured layers in the prescribed order.
func (p *PromptAssembler) Assemble() string {
	layers := []struct {
		name    string
		content string
	}{
		{"BASE", p.base},
		{"PROJECT", p.project},
		{"INSTANCE", p.instance},
		{"FRAGMENT", p.fragment},
	}

	var builder strings.Builder
	for _, layer := range layers {
		if strings.TrimSpace(layer.content) == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("=== ")
		builder.WriteString(layer.name)
		builder.WriteString(" ===\n")
		builder.WriteString(layer.content)
	}

	return builder.String()
}
