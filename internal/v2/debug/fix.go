package debug

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// AutonomousFixInput describes the context required to apply a learnable fix and rerun validation.
type AutonomousFixInput struct {
	FixCtx          *FixContext
	ValidateCtx     *ValidateContext
	LearningSpecDir string
	FailureSignal   string
	RootCause       RootCause
	Diagnosis       *Diagnosis
}

// AutonomousFixResult describes the applied fix plus learning and validation outcomes.
type AutonomousFixResult struct {
	*FixResult
	ValidateResult *ValidateResult
	LearningEntry  string
}

// ApplyAutonomousFix applies a learnable fix, persists the entry, and reruns validation.
func ApplyAutonomousFix(ctx context.Context, input *AutonomousFixInput) (*AutonomousFixResult, error) {
	if input == nil || input.FixCtx == nil {
		return nil, ErrNilFixContext
	}
	if ctx == nil {
		ctx = context.Background()
	}

	fixResult, err := ApplyFix(ctx, input.FixCtx)
	if err != nil {
		return nil, err
	}

	result := &AutonomousFixResult{FixResult: fixResult}
	learningEntry := learningEntryForAutonomousFix(input)
	specDir := strings.TrimSpace(input.LearningSpecDir)
	if specDir == "" {
		specDir = strings.TrimSpace(input.FixCtx.WorktreeRoot)
	}
	if specDir != "" && learningEntry != "" {
		if entry, persistErr := persistAutonomousLearningEntry(specDir, learningEntry); persistErr != nil {
			return nil, persistErr
		} else if entry != "" {
			result.LearningEntry = entry
		}
	}

	if input.ValidateCtx != nil {
		validateResult, validateErr := ValidateFix(ctx, input.ValidateCtx)
		if validateErr != nil {
			return nil, validateErr
		}
		result.ValidateResult = validateResult
	}

	return result, nil
}

func signalForAutonomousFix(input *AutonomousFixInput) string {
	if input == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(input.FailureSignal); trimmed != "" {
		return trimmed
	}
	if ctx := input.FixCtx; ctx != nil {
		return strings.TrimSpace(ctx.ErrorMsg)
	}
	return ""
}

func learningEntryForAutonomousFix(input *AutonomousFixInput) string {
	if input == nil {
		return ""
	}
	failureSignal := signalForAutonomousFix(input)
	learningInput := LearningExtractionInput{
		RootCause:     input.RootCause,
		FailureSignal: failureSignal,
		Diagnosis:     input.Diagnosis,
	}
	if ctx := input.FixCtx; ctx != nil {
		learningInput.ErrorText = ctx.ErrorMsg
		learningInput.StageName = ctx.FailedStage
	}
	learning := ExtractLearning(learningInput)
	if !learning.Autonomous {
		return ""
	}
	return learning.LearningsEntry
}

func persistAutonomousLearningEntry(specDir, entry string) (string, error) {
	trimmedDir := strings.TrimSpace(specDir)
	trimmedEntry := strings.TrimSpace(entry)
	if trimmedDir == "" || trimmedEntry == "" {
		return "", nil
	}
	learningsPath := filepath.Join(trimmedDir, "LEARNINGS.md")
	if err := PersistLearning(learningsPath, trimmedEntry); err != nil {
		return "", err
	}
	return trimmedEntry, nil
}

// SystemicFixInput captures the fix and diagnostic context for a systemic change.
type SystemicFixInput struct {
	FixCtx        *FixContext
	RootCause     RootCause
	FailureSignal string
}

// SystemicFixResult combines the code fix application outcome with any
// structured recommendation that requires human review.
type SystemicFixResult struct {
	*FixResult
	SystemicRecommendation string
}

// ApplySystemicFix applies the patch described in the fix context and, if the
// diff touches systemic assets, returns a recommendation for human review.
func ApplySystemicFix(ctx context.Context, input *SystemicFixInput) (*SystemicFixResult, error) {
	if input == nil {
		return nil, fmt.Errorf("systemic fix input is nil")
	}
	if input.FixCtx == nil {
		return nil, ErrNilFixContext
	}

	fixCtxCopy := *input.FixCtx
	nonSystemicPatch, nonSystemicPaths := filterNonSystemicPatchSections(fixCtxCopy.CodePatch)

	var result *FixResult
	if strings.TrimSpace(nonSystemicPatch) != "" {
		fixCtxCopy.CodePatch = nonSystemicPatch
		fixCtxCopy.FilesInvolved = filterFilesInvolvedForPaths(input.FixCtx.FilesInvolved, nonSystemicPaths)
		var err error
		result, err = ApplyFix(ctx, &fixCtxCopy)
		if err != nil {
			return nil, err
		}
	} else {
		result = &FixResult{
			ValidPath: strings.TrimSpace(fixCtxCopy.WorktreeRoot),
		}
	}

	recommendation := buildSystemicRecommendationForFix(input)
	return &SystemicFixResult{
		FixResult:              result,
		SystemicRecommendation: recommendation,
	}, nil
}

func buildSystemicRecommendationForFix(input *SystemicFixInput) string {
	if input == nil || input.FixCtx == nil {
		return ""
	}
	patch := strings.TrimSpace(input.FixCtx.CodePatch)
	categories := detectSystemicPatchCategories(patch)
	if len(categories) == 0 {
		return ""
	}

	signal := buildFailureSignal(input.FailureSignal, input.FixCtx, categories)
	if rec := BuildSystemicRecommendation(input.RootCause, signal); rec != "" {
		return rec
	}

	return fallbackSystemicRecommendation(signal, categories)
}

func buildFailureSignal(failureSignal string, fixCtx *FixContext, categories []string) string {
	if trimmed := strings.TrimSpace(failureSignal); trimmed != "" {
		return trimmed
	}
	if fixCtx != nil {
		if trimmed := strings.TrimSpace(fixCtx.ErrorMsg); trimmed != "" {
			return trimmed
		}
	}
	return failureSignalFromCategories(categories)
}

var systemicCategorySignals = map[string]string{
	systemicCategoryPromptFragment: "Systemic change touched prompt fragments that need clarity.",
	systemicCategoryGuard:          "Systemic change updated a pipeline guard that should block regressions.",
	systemicCategoryProcessRule:    "Systemic change modified process rules or workflow that require review.",
}

func failureSignalFromCategories(categories []string) string {
	if len(categories) == 0 {
		return ""
	}
	parts := make([]string, 0, len(categories))
	for _, category := range categories {
		if template, ok := systemicCategorySignals[category]; ok {
			parts = append(parts, template)
			continue
		}
		parts = append(parts, category)
	}
	return strings.Join(parts, " ")
}

func fallbackSystemicRecommendation(signal string, categories []string) string {
	categoryDesc := strings.Join(categories, ", ")
	if strings.TrimSpace(signal) == "" {
		signal = fmt.Sprintf("Patch updates %s assets that require human review.", categoryDesc)
	}

	return fmt.Sprintf(
		"Systemic recommendation for human review: patch touches %s. Rationale: %s Awaiting human approval before applying these changes.",
		categoryDesc,
		signal,
	)
}

type patchSection struct {
	path    string
	content string
}

func filterNonSystemicPatchSections(patch string) (string, []string) {
	if strings.TrimSpace(patch) == "" {
		return "", nil
	}
	sections := splitPatchSections(patch)
	var filtered []patchSection
	seenPaths := map[string]struct{}{}

	for _, section := range sections {
		if section.path == "" || isSystemicPath(section.path) {
			continue
		}
		filtered = append(filtered, section)
		seenPaths[section.path] = struct{}{}
	}

	if len(filtered) == 0 {
		return "", nil
	}

	paths := make([]string, 0, len(seenPaths))
	for path := range seenPaths {
		paths = append(paths, path)
	}

	return buildPatchFromSections(filtered), paths
}

func splitPatchSections(patch string) []patchSection {
	normalized := strings.ReplaceAll(patch, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	var sections []patchSection
	var current []string
	var currentPath string
	started := false

	flush := func() {
		if !started {
			return
		}
		sections = append(sections, patchSection{
			path:    currentPath,
			content: strings.Join(current, "\n"),
		})
		current = nil
		currentPath = ""
		started = false
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			current = []string{line}
			currentPath = extractPatchPathFromDiffLine(line)
			started = true
			continue
		}
		if started {
			current = append(current, line)
		}
	}
	flush()

	return sections
}

func buildPatchFromSections(sections []patchSection) string {
	var builder strings.Builder
	for i, sect := range sections {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(sect.content)
	}
	if builder.Len() == 0 {
		return ""
	}
	builder.WriteString("\n")
	return builder.String()
}

func extractPatchPathFromDiffLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	path := strings.TrimSpace(fields[3])
	path = strings.TrimPrefix(path, "b/")
	path = strings.TrimPrefix(path, "a/")
	return path
}

func filterFilesInvolvedForPaths(files, patchPaths []string) []string {
	if len(files) == 0 || len(patchPaths) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(files))
	for _, f := range files {
		if patchTouchesFile(f, patchPaths) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

func isSystemicPath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if normalized == "" {
		return false
	}
	return isPromptFragmentPath(normalized) || isGuardPath(normalized) || isProcessRulePath(normalized)
}
