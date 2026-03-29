package prompt

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// scopedPackagePattern matches package paths like internal/foo/bar or cmd/gromit.
var scopedPackagePattern = regexp.MustCompile(`(?:internal|cmd|pkg|src)/[A-Za-z0-9._/\-]+`)

// phaseMaxChars defines per-phase character caps for RULES.md sections.
var phaseMaxChars = map[string]int{
	"build":     12800,
	"red":       8500,
	"green":     8500,
	"refactor":  8500,
	"plan":      12800,
	"decompose": 12800,
	"accept":    8500,
	"validate":  8500,
	"review":    10000,
	"triage":    8500,
}

// fileExtensions lists common file extensions used to identify file-path tokens.
var fileExtensions = []string{
	".go", ".yaml", ".yml", ".json", ".md", ".ts", ".js", ".py",
	".toml", ".cfg", ".sh", ".bash", ".txt", ".html", ".css",
	".rs", ".java", ".c", ".h", ".cpp", ".rb", ".proto",
}

// dirPrefixes lists directory prefixes that indicate a token is a file path.
var dirPrefixes = []string{
	"internal/", "cmd/", "pkg/", "src/", "./", "../", "vendor/",
	"test/", "tests/", "docs/", "lib/", "bin/", "config/",
}

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
	MaxChars   int // Optional: if > 0, overrides DefaultMaxChars in ShapeBudget.
}

// NewPromptAssembler returns an assembler initialized with the provided layers.
func NewPromptAssembler(base, project, instance, fragment string) *PromptAssembler {
	return &PromptAssembler{base: base, project: project, instance: instance, fragment: fragment}
}

// LastReport returns the ShapeReport from the most recent Assemble call, or nil.
func (p *PromptAssembler) LastReport() *ShapeReport {
	return p.lastReport
}

// parsePhaseAnnotation extracts phase names from a "<!-- phases: x, y, z -->" annotation.
// Returns nil if no annotation is found.
func parsePhaseAnnotation(heading string) []string {
	const prefix = "<!-- phases:"
	const suffix = "-->"
	start := strings.Index(heading, prefix)
	if start == -1 {
		return nil
	}
	end := strings.Index(heading[start:], suffix)
	if end == -1 {
		return nil
	}
	inner := heading[start+len(prefix) : start+end]
	parts := strings.Split(inner, ",")
	var phases []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			phases = append(phases, strings.ToLower(trimmed))
		}
	}
	return phases
}

// loadBaseInstructions returns base instructions filtered by phase.
// When phase is empty, the full base text is returned unmodified.
// It first checks for <!-- phases: ... --> annotations on ## headings.
// If any annotations exist, annotation-based filtering is used.
// Otherwise, it falls back to legacy exact-heading matching.
func (p *PromptAssembler) loadBaseInstructions(phase string) string {
	if phase == "" {
		return p.base
	}

	normalizedPhase := strings.ToLower(phase)

	lines := strings.Split(p.base, "\n")
	type section struct {
		start      int
		end        int
		hasAnnot   bool
		matchPhase bool
	}
	var sections []section
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		phases := parsePhaseAnnotation(line)
		hasAnnot := phases != nil
		matchPhase := false
		if hasAnnot {
			for _, ph := range phases {
				if ph == normalizedPhase {
					matchPhase = true
					break
				}
			}
		}
		sections = append(sections, section{start: i, hasAnnot: hasAnnot, matchPhase: matchPhase})
	}

	for i := range sections {
		if i+1 < len(sections) {
			sections[i].end = sections[i+1].start
		} else {
			sections[i].end = len(lines)
		}
	}

	anyAnnotated := false
	for _, s := range sections {
		if s.hasAnnot {
			anyAnnotated = true
			break
		}
	}

	if !anyAnnotated {
		return p.loadBaseInstructionsLegacy(phase)
	}

	var builder strings.Builder
	for _, s := range sections {
		if s.hasAnnot && !s.matchPhase {
			continue
		}
		chunk := strings.Join(lines[s.start:s.end], "\n")
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(chunk)
	}

	var preamble string
	if len(sections) > 0 && sections[0].start > 0 {
		candidate := strings.Join(lines[:sections[0].start], "\n")
		if strings.TrimSpace(candidate) != "" {
			preamble = candidate
		}
	}

	body := builder.String()
	if body == "" {
		if preamble == "" {
			return ""
		}
		return p.applyPhaseCap(normalizedPhase, preamble)
	}
	return p.applyPhaseCapForPhaseSections(normalizedPhase, preamble, body)
}

// loadBaseInstructionsLegacy uses exact heading matching for phase filtering.
// This is the fallback when no <!-- phases: ... --> annotations are found.
func (p *PromptAssembler) loadBaseInstructionsLegacy(phase string) string {
	marker := "## " + phase
	idx := strings.Index(p.base, marker)
	// Verify the match is an exact heading (next char must be whitespace, newline, or end of string).
	for idx != -1 {
		end := idx + len(marker)
		if end >= len(p.base) || p.base[end] == '\n' || p.base[end] == '\r' || p.base[end] == ' ' || p.base[end] == '\t' {
			break
		}
		// Partial match (e.g. "## builder" for phase "build"); keep searching.
		next := strings.Index(p.base[end:], marker)
		if next == -1 {
			idx = -1
		} else {
			idx = end + next
		}
	}
	if idx == -1 {
		// No phase-specific section found; return full base.
		return p.base
	}
	// Extract from the marker to the next "## " heading or end of string.
	rest := p.base[idx:]
	nextIdx := strings.Index(rest[len(marker):], "\n## ")
	var section string
	if nextIdx == -1 {
		section = rest
	} else {
		section = rest[:len(marker)+nextIdx]
	}
	return p.applyPhaseCap(phase, section)
}

// applyPhaseCap truncates the section to the per-phase character cap if defined.
func (p *PromptAssembler) applyPhaseCap(phase, section string) string {
	phaseKey := strings.ToLower(phase)
	if cap, ok := phaseMaxChars[phaseKey]; ok && len(section) > cap {
		section = section[:cap]
		// Back up to valid UTF-8 boundary.
		for len(section) > 0 && !utf8.Valid([]byte(section)) {
			section = section[:len(section)-1]
		}
	}
	return section
}

func (p *PromptAssembler) applyPhaseCapForPhaseSections(phase, preamble, sections string) string {
	phaseKey := strings.ToLower(phase)
	capVal, ok := phaseMaxChars[phaseKey]
	if !ok || capVal <= 0 {
		return joinPreambleAndBody(preamble, sections)
	}
	if len(sections) >= capVal {
		return trimUTF8Prefix(sections, capVal)
	}
	remaining := capVal - len(sections)
	maxPreamble := remaining
	if preamble != "" && maxPreamble > 0 {
		maxPreamble--
	}
	if maxPreamble < 0 {
		maxPreamble = 0
	}
	preambleTail := trimUTF8Suffix(preamble, maxPreamble)
	if preambleTail == "" {
		return sections
	}
	return preambleTail + "\n" + sections
}

func joinPreambleAndBody(preamble, body string) string {
	if preamble == "" {
		return body
	}
	if body == "" {
		return preamble
	}
	return preamble + "\n" + body
}

func trimUTF8Prefix(s string, limit int) string {
	if limit >= len(s) {
		return s
	}
	trimmed := s[:limit]
	for len(trimmed) > 0 && !utf8.Valid([]byte(trimmed)) {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func trimUTF8Suffix(s string, limit int) string {
	if limit >= len(s) {
		return s
	}
	start := len(s) - limit
	if start < 0 {
		start = 0
	}
	trimmed := s[start:]
	for len(trimmed) > 0 && !utf8.Valid([]byte(trimmed)) {
		trimmed = trimmed[1:]
	}
	return trimmed
}

// loadProjectContext returns project context scoped to the bead's files.
// When the bead has no files, the full project text is returned unmodified.
func (p *PromptAssembler) loadProjectContext(bead BeadInfo) string {
	beadPaths := extractPackagePaths(bead.Title, bead.Files)
	if len(bead.Files) == 0 && bead.Title == "" {
		return p.project
	}

	result := p.project

	// File-path token filtering: only include lines that reference bead files
	// or non-file-specific lines. This is a best-effort filter.
	if len(bead.Files) > 0 {
		lines := strings.Split(result, "\n")
		var filtered []string
		for _, line := range lines {
			relevant := true
			if containsFilePathToken(line) {
				relevant = false
				for _, f := range bead.Files {
					if strings.Contains(line, f) {
						relevant = true
						break
					}
				}
				if !relevant && len(beadPaths) > 0 && bulletMatchesAnyPath(line, beadPaths) {
					relevant = true
				}
			}
			if relevant {
				filtered = append(filtered, line)
			}
		}
		result = strings.Join(filtered, "\n")
	}

	if len(beadPaths) > 0 {
		result = scopeArchitectureSection(result, beadPaths)
	}

	return result
}

// extractPackagePaths discovers package directory paths from the bead title and file list.
// It scans the title for tokens matching known directory prefixes or the scopedPackagePattern,
// and extracts directory portions of files. Returns deduplicated sorted paths.
func extractPackagePaths(title string, files []string) []string {
	unique := make(map[string]struct{})

	// Extract package paths from the title using the scoped package regex.
	matches := scopedPackagePattern.FindAllString(title, -1)
	for _, match := range matches {
		dir := normalizePackagePath(match)
		if dir != "" {
			unique[dir] = struct{}{}
		}
	}

	// Also scan title tokens for dirPrefix matches.
	for _, tok := range strings.Fields(title) {
		for _, prefix := range dirPrefixes {
			if strings.HasPrefix(tok, prefix) {
				dir := normalizePackagePath(tok)
				if dir != "" {
					unique[dir] = struct{}{}
				}
				break
			}
		}
	}

	// Extract directory paths from bead files.
	for _, f := range files {
		dir := filepath.Dir(f)
		if dir == "." || dir == "" {
			continue
		}
		// Normalize to forward slashes for matching.
		dir = filepath.ToSlash(dir)
		unique[dir] = struct{}{}
	}

	if len(unique) == 0 {
		return nil
	}

	paths := make([]string, 0, len(unique))
	for p := range unique {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// normalizePackagePath trims a discovered path token to its directory portion,
// stripping trailing file names and normalizing slashes.
func normalizePackagePath(path string) string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return ""
	}

	// Strip trailing file name (has extension).
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if strings.Contains(last, ".") {
			parts = parts[:len(parts)-1]
		}
	}
	// Strip trailing "...".
	if len(parts) > 0 && parts[len(parts)-1] == "..." {
		parts = parts[:len(parts)-1]
	}

	if len(parts) < 2 {
		return ""
	}

	return strings.Join(parts, "/")
}

// scopeArchitectureSection filters the ## Architecture section of project text
// to only include bullet points that reference one of the given package paths.
// Non-architecture content is preserved unchanged.
func scopeArchitectureSection(project string, beadPaths []string) string {
	lines := strings.Split(project, "\n")

	// Find the "## Architecture" section boundaries.
	archStart := -1
	archEnd := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		if archStart == -1 {
			if strings.HasPrefix(trimmed, "## architecture") {
				archStart = i
			}
		} else if strings.HasPrefix(trimmed, "## ") {
			archEnd = i
			break
		}
	}

	if archStart == -1 {
		return project
	}

	// Filter bullets within the architecture section.
	var result []string
	// Keep everything before the architecture section.
	result = append(result, lines[:archStart]...)
	// Keep the architecture heading itself.
	result = append(result, lines[archStart])

	for i := archStart + 1; i < archEnd; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		// Keep non-bullet lines (blank lines, sub-headings, prose).
		if !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
			result = append(result, line)
			continue
		}
		// For bullet lines, keep if any beadPath is referenced.
		if bulletMatchesAnyPath(line, beadPaths) {
			result = append(result, line)
		}
	}

	// Keep everything after the architecture section.
	result = append(result, lines[archEnd:]...)

	return strings.Join(result, "\n")
}

// bulletMatchesAnyPath returns true if the bullet line references any of the given paths.
func bulletMatchesAnyPath(line string, paths []string) bool {
	for _, p := range paths {
		for _, candidate := range pathCandidates(p) {
			if candidateMatchesLine(line, candidate) {
				return true
			}
		}
	}
	return false
}

func candidateMatchesLine(line, candidate string) bool {
	if candidate == "" {
		return false
	}
	if strings.Contains(line, candidate) {
		return true
	}
	lastSegment := candidate
	if idx := strings.LastIndex(candidate, "/"); idx != -1 {
		lastSegment = candidate[idx+1:]
	}
	if lastSegment != "" && strings.Contains(line, "`"+lastSegment+"/`") {
		return true
	}
	return false
}

func pathCandidates(path string) []string {
	var candidates []string
	current := path
	for current != "" {
		if strings.Contains(current, "/") {
			candidates = append(candidates, current)
		}
		idx := strings.LastIndex(current, "/")
		if idx == -1 {
			break
		}
		current = current[:idx]
	}
	return candidates
}

// Assemble concatenates the configured layers, applies shaping, and records the last report.
func (p *PromptAssembler) Assemble(phase string, bead BeadInfo) string {
	shaped, report := p.assemblePromptInternal(phase, bead)
	p.lastReport = &report
	return shaped
}

// AssembleWithReport is like Assemble but also returns the shaping report.
func (p *PromptAssembler) AssembleWithReport(phase string, bead BeadInfo) (string, *ShapeReport) {
	shaped, report := p.assemblePromptInternal(phase, bead)
	p.lastReport = &report
	return shaped, &report
}

func (p *PromptAssembler) assemblePromptInternal(phase string, bead BeadInfo) (string, ShapeReport) {
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

	budget := DefaultMaxChars
	if p.MaxChars > 0 {
		budget = p.MaxChars
	}
	shaped, report := ShapeBudget(raw, fileCount, budget)

	return shaped, report
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
	adjustedBudget := scopeAdjustedBudget(maxChars, fileCount)

	if adjustedBudget < 1 {
		adjustedBudget = 1
	}

	if adjustedBudget > maxChars {
		adjustedBudget = maxChars
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
	// Back up to valid UTF-8 boundary to avoid splitting multi-byte characters.
	for len(shaped) > 0 && !utf8.Valid([]byte(shaped)) {
		shaped = shaped[:len(shaped)-1]
	}
	report.ShapedSize = len(shaped)
	report.Trimmed = true
	report.TrimmedBytes = len(total) - len(shaped)
	return shaped, report
}

func scopeAdjustedBudget(maxChars, fileCount int) int {
	switch {
	case fileCount >= 1 && fileCount <= 2:
		return maxChars / 2
	case fileCount >= 3 && fileCount <= 4:
		return maxChars * 3 / 4
	default:
		return maxChars
	}
}

// containsFilePathToken returns true if the line contains a token that looks
// like a file path (has a known directory prefix or file extension).
func containsFilePathToken(line string) bool {
	tokens := strings.Fields(line)
	for _, tok := range tokens {
		if !strings.Contains(tok, "/") {
			continue
		}
		// Check for known directory prefixes.
		for _, prefix := range dirPrefixes {
			if strings.HasPrefix(tok, prefix) {
				return true
			}
		}
		// Check for file extension.
		ext := filepath.Ext(tok)
		for _, fe := range fileExtensions {
			if ext == fe {
				return true
			}
		}
	}
	return false
}
