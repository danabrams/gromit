package prompt

import "strings"

const (
	rulesSectionHeaderPrefix = "## "

	// Phase annotation delimiters in RULES.md section headers.
	phaseAnnotationPrefix = "<!-- phases:"
	phaseAnnotationSuffix = "-->"
)

// filterRulesByPhase parses RULES.md content, filters ## sections by phase annotations,
// and strips annotation comments from the output.
func filterRulesByPhase(content, phase string) string {
	lines := strings.Split(content, "\n")

	type section struct {
		headerLine string   // The ## header line (with annotation)
		bodyLines  []string // Lines after the header until the next ## header
		phases     []string // Parsed phases from annotation (nil = all phases)
	}

	var preamble []string // Lines before the first ## header
	var sections []section
	var current *section

	for _, line := range lines {
		if strings.HasPrefix(line, rulesSectionHeaderPrefix) {
			// Save previous section
			if current != nil {
				sections = append(sections, *current)
			}
			// Parse phase annotation from the header
			phases := parsePhaseAnnotation(line)
			current = &section{
				headerLine: line,
				phases:     phases,
			}
		} else if current != nil {
			current.bodyLines = append(current.bodyLines, line)
		} else {
			preamble = append(preamble, line)
		}
	}
	// Don't forget the last section
	if current != nil {
		sections = append(sections, *current)
	}

	// Build output: preamble + matching sections
	var out strings.Builder
	for _, line := range preamble {
		out.WriteString(line)
		out.WriteString("\n")
	}

	for _, s := range sections {
		if !sectionMatchesPhase(s.phases, phase) {
			continue
		}
		// Write header with annotation stripped
		out.WriteString(stripPhaseAnnotation(s.headerLine))
		out.WriteString("\n")
		for _, line := range s.bodyLines {
			out.WriteString(line)
			out.WriteString("\n")
		}
	}

	// Trim trailing newline to match input convention
	result := out.String()
	if strings.HasSuffix(result, "\n") {
		result = result[:len(result)-1]
	}
	return result
}

// parsePhaseAnnotation extracts phase names from a <!-- phases: build, review --> comment
// in a header line. Returns nil if no annotation is found (meaning all phases).
func parsePhaseAnnotation(headerLine string) []string {
	idx, endIdx, ok := findPhaseAnnotationBounds(headerLine)
	if !ok {
		return nil
	}
	phaseStr := headerLine[idx+len(phaseAnnotationPrefix) : endIdx-len(phaseAnnotationSuffix)]
	parts := strings.Split(phaseStr, ",")
	var phases []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			phases = append(phases, p)
		}
	}
	return phases
}

// sectionMatchesPhase returns true if the section should be included for the given phase.
// If phases is nil (no annotation), the section is included in all phases.
func sectionMatchesPhase(phases []string, phase string) bool {
	if phases == nil {
		return true
	}
	for _, p := range phases {
		if p == phase {
			return true
		}
	}
	return false
}

// stripPhaseAnnotation removes the <!-- phases: ... --> comment from a header line.
func stripPhaseAnnotation(headerLine string) string {
	idx, endIdx, ok := findPhaseAnnotationBounds(headerLine)
	if !ok {
		return headerLine
	}
	// Strip the annotation and any trailing whitespace
	before := strings.TrimRight(headerLine[:idx], " ")
	after := headerLine[endIdx:]
	return before + after
}

func findPhaseAnnotationBounds(headerLine string) (int, int, bool) {
	idx := strings.Index(headerLine, phaseAnnotationPrefix)
	if idx < 0 {
		return 0, 0, false
	}
	relativeEndIdx := strings.Index(headerLine[idx:], phaseAnnotationSuffix)
	if relativeEndIdx < 0 {
		return 0, 0, false
	}
	endIdx := idx + relativeEndIdx + len(phaseAnnotationSuffix)
	return idx, endIdx, true
}
