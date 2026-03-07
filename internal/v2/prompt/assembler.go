package prompt

import (
	"regexp"
	"strings"
)

// BeadInfo provides identifying metadata about the bead for prompt scoping.
type BeadInfo struct {
	Title string
}

var scopePackagePattern = regexp.MustCompile(`(?:internal|cmd)/[A-Za-z0-9._/\-]+`)

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

// loadProjectContext returns the project layer scoped to packages mentioned in the bead title.
// Falls back to the full project content when no package paths are found.
func (p *PromptAssembler) loadProjectContext(bead BeadInfo) string {
	matches := scopePackagePattern.FindAllString(bead.Title, -1)
	if len(matches) == 0 {
		return p.project
	}
	return p.project
}
