package prompt

import (
	"regexp"
	"sort"
	"strings"
)

const (
	rulesSectionHeaderPrefix = "## "
	phaseAnnotationPrefix    = "<!-- phases:"
	phaseAnnotationSuffix    = "-->"
)

// rulesPhaseMaxChars maps phase names to their character caps for the base rules layer.
var rulesPhaseMaxChars = map[string]int{
	"build":    12800,
	"red":      8500,
	"green":    8500,
	"refactor": 8500,
	"review":   8500,
	"plan":     5000,
	"validate": 5000,
}

// loadBaseInstructions returns the base layer filtered to phase-relevant sections
// and capped at the phase-specific character limit.
// Returns the full base content if phase is empty.
func (p *PromptAssembler) loadBaseInstructions(phase string) string {
	if phase == "" {
		return p.base
	}
	filtered := filterRulesByPhase(p.base, phase)
	if cap, ok := rulesPhaseMaxChars[strings.ToLower(phase)]; ok && len(filtered) > cap {
		filtered = filtered[:cap]
	}
	return filtered
}

// filterRulesByPhase parses RULES.md content, filters ## sections by phase annotations,
// and strips annotation comments from the output.
func filterRulesByPhase(content, phase string) string {
	lines := strings.Split(content, "\n")

	type section struct {
		headerLine string
		bodyLines  []string
		phases     []string // nil = all phases
	}

	var preamble []string
	var sections []section
	var current *section

	for _, line := range lines {
		if strings.HasPrefix(line, rulesSectionHeaderPrefix) {
			if current != nil {
				sections = append(sections, *current)
			}
			phases := parseRulesPhaseAnnotation(line)
			current = &section{headerLine: line, phases: phases}
		} else if current != nil {
			current.bodyLines = append(current.bodyLines, line)
		} else {
			preamble = append(preamble, line)
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}

	var out strings.Builder
	for _, line := range preamble {
		out.WriteString(line)
		out.WriteString("\n")
	}
	for _, s := range sections {
		if !rulesPhaseMatches(s.phases, phase) {
			continue
		}
		out.WriteString(stripRulesPhaseAnnotation(s.headerLine))
		out.WriteString("\n")
		for _, line := range s.bodyLines {
			out.WriteString(line)
			out.WriteString("\n")
		}
	}

	result := out.String()
	if strings.HasSuffix(result, "\n") {
		result = result[:len(result)-1]
	}
	return result
}

func parseRulesPhaseAnnotation(headerLine string) []string {
	idx := strings.Index(headerLine, phaseAnnotationPrefix)
	if idx < 0 {
		return nil
	}
	rest := headerLine[idx+len(phaseAnnotationPrefix):]
	endIdx := strings.Index(rest, phaseAnnotationSuffix)
	if endIdx < 0 {
		return nil
	}
	parts := strings.Split(rest[:endIdx], ",")
	var phases []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			phases = append(phases, p)
		}
	}
	return phases
}

func rulesPhaseMatches(phases []string, phase string) bool {
	if phases == nil {
		return true
	}
	phase = strings.ToLower(phase)
	for _, p := range phases {
		if p == phase {
			return true
		}
	}
	return false
}

func stripRulesPhaseAnnotation(headerLine string) string {
	idx := strings.Index(headerLine, phaseAnnotationPrefix)
	if idx < 0 {
		return headerLine
	}
	rest := headerLine[idx:]
	endIdx := strings.Index(rest, phaseAnnotationSuffix)
	if endIdx < 0 {
		return headerLine
	}
	before := strings.TrimRight(headerLine[:idx], " ")
	after := rest[endIdx+len(phaseAnnotationSuffix):]
	return before + after
}

// BeadInfo provides identifying metadata about the bead for prompt scoping.
type BeadInfo struct {
	Title string
}

var scopePackagePattern = regexp.MustCompile(`(?:internal|cmd)/[A-Za-z0-9._/\-]+`)
var h2SectionPattern = regexp.MustCompile(`(?m)^##\s+([^\n#][^\n]*)\s*$`)
var archBulletPattern = regexp.MustCompile("(?m)^\\s*-\\s*`([^`]+)`\\s*(?:—|-)\\s*(.+?)\\s*$")

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
// Uses v1's applyScopedClaudeContext algorithm: extract paths from title, scope Architecture
// section, preserve Key Principles. Falls back to full content when no paths found or
// when CLAUDE.md lacks both Architecture and Key Principles sections.
func (p *PromptAssembler) loadProjectContext(bead BeadInfo) string {
	matches := scopePackagePattern.FindAllString(bead.Title, -1)
	if len(matches) == 0 {
		return p.project
	}

	unique := map[string]struct{}{}
	for _, m := range matches {
		if n := normalizePkgPath(m); n != "" {
			unique[n] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return p.project
	}

	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return scopeProjectContent(p.project, paths)
}

// normalizePkgPath normalizes a package path to a trailing-slash form.
func normalizePkgPath(path string) string {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	// Drop file-like last segments (contain a dot).
	if len(parts) > 0 && strings.Contains(parts[len(parts)-1], ".") {
		parts = parts[:len(parts)-1]
	}
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts, "/") + "/"
}

// scopeProjectContent scopes the Architecture section of CLAUDE.md to only include
// paths that match the given package paths. Preserves Key Principles verbatim.
// Returns content unchanged if both Architecture and Key Principles sections are not present.
func scopeProjectContent(content string, paths []string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	matches := h2SectionPattern.FindAllStringSubmatchIndex(normalized, -1)
	if len(matches) == 0 {
		return content
	}

	type section struct {
		name string
		raw  string
		body string
	}

	preamble := strings.TrimSpace(normalized[:matches[0][0]])
	sections := make([]section, 0, len(matches))
	for i, match := range matches {
		headerEnd := match[1]
		nameStart, nameEnd := match[2], match[3]
		sectionEnd := len(normalized)
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		}
		name := strings.ToLower(strings.TrimSpace(normalized[nameStart:nameEnd]))
		raw := strings.Trim(normalized[match[0]:sectionEnd], "\n")
		body := strings.Trim(normalized[headerEnd:sectionEnd], "\n")
		sections = append(sections, section{name: name, raw: raw, body: body})
	}

	var archSection, keySection *section
	for i := range sections {
		switch sections[i].name {
		case "architecture":
			archSection = &sections[i]
		case "key principles":
			keySection = &sections[i]
		}
	}
	if archSection == nil || keySection == nil {
		return content
	}

	// Filter architecture bullets to only those matching the requested paths.
	bulletMatches := archBulletPattern.FindAllStringSubmatch(archSection.body, -1)
	filtered := make([]string, 0, len(bulletMatches))
	for _, bm := range bulletMatches {
		if len(bm) < 3 {
			continue
		}
		entryPath := normalizePkgPath(bm[1])
		for _, want := range paths {
			if strings.HasPrefix(entryPath, want) || strings.HasPrefix(want, entryPath) {
				filtered = append(filtered, "- `"+strings.Trim(bm[1], "/")+"/`"+" — "+strings.TrimSpace(bm[2]))
				break
			}
		}
	}

	archContent := "## Architecture"
	if len(filtered) > 0 {
		archContent += "\n\n" + strings.Join(filtered, "\n")
	}

	parts := make([]string, 0, 3)
	if preamble != "" {
		parts = append(parts, preamble)
	}
	parts = append(parts, archContent, keySection.raw)
	return strings.Join(parts, "\n\n")
}
