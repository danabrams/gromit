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
	var builder strings.Builder
	appendLayer := func(layer string) {
		if layer == "" {
			return
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(layer)
	}

	appendLayer(p.base)
	appendLayer(p.project)
	appendLayer(p.instance)
	appendLayer(p.fragment)

	return builder.String()
}
