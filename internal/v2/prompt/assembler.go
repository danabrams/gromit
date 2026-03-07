package prompt

import "strings"

// DefaultMaxChars is the default maximum character budget for shaped prompts.
const DefaultMaxChars = 100000

// BeadInfo carries bead metadata used for prompt shaping.
type BeadInfo struct {
	Title     string
	Files     []string
	FileCount int
}

// ShapeReport records budget shaping decisions for observability.
type ShapeReport struct {
	OriginalSize   int
	ShapedSize     int
	MaxBudget      int
	AdjustedBudget int
	FileCount      int
	Trimmed        bool
	TrimmedBytes   int
}

// PromptAssembler merges prompt layers into a single payload.
type PromptAssembler struct {
	base       string
	project    string
	instance   string
	fragment   string
	lastReport *ShapeReport
}

// NewPromptAssembler returns an assembler initialized with the provided layers.
func NewPromptAssembler(base, project, instance, fragment string) *PromptAssembler {
	return &PromptAssembler{base: base, project: project, instance: instance, fragment: fragment}
}

// LastReport returns the ShapeReport from the most recent Assemble call, or nil.
func (p *PromptAssembler) LastReport() *ShapeReport {
	return p.lastReport
}

// loadBaseInstructions returns base instructions filtered by phase.
// When phase is empty, the full base text is returned unmodified.
func (p *PromptAssembler) loadBaseInstructions(phase string) string {
	if phase == "" {
		return p.base
	}
	// Phase-specific filtering: look for phase markers and extract the relevant section.
	marker := "## " + phase
	idx := strings.Index(p.base, marker)
	if idx == -1 {
		// No phase-specific section found; return full base.
		return p.base
	}
	// Extract from the marker to the next "## " heading or end of string.
	rest := p.base[idx:]
	nextIdx := strings.Index(rest[len(marker):], "\n## ")
	if nextIdx == -1 {
		return rest
	}
	return rest[:len(marker)+nextIdx]
}

// loadProjectContext returns project context scoped to the bead's files.
// When the bead has no files, the full project text is returned unmodified.
func (p *PromptAssembler) loadProjectContext(bead BeadInfo) string {
	if len(bead.Files) == 0 {
		return p.project
	}
	// Scope project context: only include lines that reference bead files
	// or non-file-specific lines. This is a best-effort filter.
	lines := strings.Split(p.project, "\n")
	var filtered []string
	for _, line := range lines {
		relevant := true
		// If the line looks like a file-scoped entry (contains a path-like token),
		// check if it references one of the bead's files.
		if strings.Contains(line, "/") {
			relevant = false
			for _, f := range bead.Files {
				if strings.Contains(line, f) {
					relevant = true
					break
				}
			}
		}
		if relevant {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// Assemble concatenates the configured layers in the prescribed order,
// applying phase filtering, bead scoping, and budget shaping.
func (p *PromptAssembler) Assemble(phase string, bead BeadInfo) string {
	base := p.loadBaseInstructions(phase)
	project := p.loadProjectContext(bead)

	layers := []struct {
		name    string
		content string
	}{
		{"BASE", base},
		{"PROJECT", project},
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

	raw := builder.String()

	fileCount := bead.FileCount
	if fileCount == 0 {
		fileCount = len(bead.Files)
	}

	shaped, report := ShapeBudget(raw, fileCount, DefaultMaxChars)
	p.lastReport = &report

	return shaped
}

// ShapeBudget applies scope-adjusted truncation to a prompt string.
//
// Scaling rules based on fileCount:
//
//	1-2 files  → 50% of maxChars
//	3-4 files  → 75% of maxChars
//	5+ files or 0 (unknown) → 100% of maxChars
//
// When the prompt exceeds the adjusted budget, context (tail) is truncated
// to fit while preserving instructions (head).
func ShapeBudget(total string, fileCount int, maxChars int) (string, ShapeReport) {
	adjustedBudget := maxChars
	switch {
	case fileCount >= 1 && fileCount <= 2:
		adjustedBudget = maxChars / 2
	case fileCount >= 3 && fileCount <= 4:
		adjustedBudget = maxChars * 3 / 4
	}

	report := ShapeReport{
		OriginalSize:   len(total),
		MaxBudget:      maxChars,
		AdjustedBudget: adjustedBudget,
		FileCount:      fileCount,
	}

	if len(total) <= adjustedBudget {
		report.ShapedSize = len(total)
		report.Trimmed = false
		report.TrimmedBytes = 0
		return total, report
	}

	// Truncate from the end (context) to preserve instructions (head).
	shaped := total[:adjustedBudget]
	report.ShapedSize = adjustedBudget
	report.Trimmed = true
	report.TrimmedBytes = len(total) - adjustedBudget
	return shaped, report
}
