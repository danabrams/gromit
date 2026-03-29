package prompt

import (
	"strings"
	"testing"
)

func TestPromptAssemblerAddsLayerMarkers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "project layer", "instance layer", "fragment layer")
	output := assembler.Assemble("", BeadInfo{})

	expectedSequence := []string{
		"=== BASE ===",
		"base layer",
		"=== PROJECT ===",
		"project layer",
		"=== INSTANCE ===",
		"instance layer",
		"=== FRAGMENT ===",
		"fragment layer",
	}

	lastIndex := 0
	for _, fragment := range expectedSequence {
		idx := strings.Index(output, fragment)
		if idx == -1 {
			t.Fatalf("output missing %q", fragment)
		}
		if idx < lastIndex {
			t.Fatalf("%q appears out of order", fragment)
		}
		lastIndex = idx
	}
}

func TestPromptAssemblerSkipsEmptyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "", "instance layer", "")
	output := assembler.Assemble("", BeadInfo{})

	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when empty")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section should be omitted when empty")
	}
}

func TestPromptAssemblerSkipsWhitespaceOnlyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "  \n\t", "instance layer", "fragment layer")
	output := assembler.Assemble("", BeadInfo{})

	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when it contains only whitespace")
	}
	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if !strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section missing")
	}
}

func TestShapeBudgetSmallBeadGets50Percent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	shaped, report := ShapeBudget(content, 1, 10000)

	if len(shaped) > 5000 {
		t.Fatalf("shaped len = %d, want <= 5000 (50%% of 10000)", len(shaped))
	}
	if !report.Trimmed {
		t.Fatal("expected Trimmed = true")
	}
	if report.AdjustedBudget != 5000 {
		t.Fatalf("AdjustedBudget = %d, want 5000", report.AdjustedBudget)
	}
}

func TestShapeBudgetMediumBeadGets75Percent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	shaped, report := ShapeBudget(content, 3, 10000)

	if len(shaped) > 7500 {
		t.Fatalf("shaped len = %d, want <= 7500 (75%% of 10000)", len(shaped))
	}
	if !report.Trimmed {
		t.Fatal("expected Trimmed = true")
	}
	if report.AdjustedBudget != 7500 {
		t.Fatalf("AdjustedBudget = %d, want 7500", report.AdjustedBudget)
	}
}

func TestShapeBudgetLargeBeadGets100Percent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	shaped, report := ShapeBudget(content, 6, 10000)

	if len(shaped) != 10000 {
		t.Fatalf("shaped len = %d, want 10000 (100%% budget, no trim needed)", len(shaped))
	}
	if report.Trimmed {
		t.Fatal("expected Trimmed = false for large bead within budget")
	}
	if report.AdjustedBudget != 10000 {
		t.Fatalf("AdjustedBudget = %d, want 10000", report.AdjustedBudget)
	}
}

func TestShapeBudgetReportsTrimmingDetails(t *testing.T) {
	content := strings.Repeat("x", 10000)
	_, report := ShapeBudget(content, 2, 10000)

	if report.OriginalSize != 10000 {
		t.Fatalf("OriginalSize = %d, want 10000", report.OriginalSize)
	}
	if report.ShapedSize != 5000 {
		t.Fatalf("ShapedSize = %d, want 5000", report.ShapedSize)
	}
	if report.TrimmedBytes != 5000 {
		t.Fatalf("TrimmedBytes = %d, want 5000", report.TrimmedBytes)
	}
	if report.FileCount != 2 {
		t.Fatalf("FileCount = %d, want 2", report.FileCount)
	}
}

func TestAssembleWithPhaseFiltersBase(t *testing.T) {
	base := "## build\nbuild instructions\n\n## validate\nvalidate instructions"
	assembler := NewPromptAssembler(base, "project context", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "build instructions") {
		t.Fatal("output should contain build instructions")
	}
	if strings.Contains(output, "validate instructions") {
		t.Fatal("output should NOT contain validate instructions when phase=build")
	}
}

func TestAssembleStoresLastReport(t *testing.T) {
	assembler := NewPromptAssembler("base", "project", "", "")
	assembler.Assemble("", BeadInfo{FileCount: 1})

	report := assembler.LastReport()
	if report == nil {
		t.Fatal("LastReport should not be nil after Assemble")
	}
	if report.FileCount != 1 {
		t.Fatalf("report.FileCount = %d, want 1", report.FileCount)
	}
}

func TestMaxCharsConfigurable(t *testing.T) {
	assembler := NewPromptAssembler(strings.Repeat("x", 5000), "", "", "")
	assembler.MaxChars = 2000
	output := assembler.Assemble("", BeadInfo{FileCount: 10})
	if len(output) > 2000 {
		t.Fatalf("output len = %d, want <= 2000 with custom MaxChars", len(output))
	}
	report := assembler.LastReport()
	if report.MaxBudget != 2000 {
		t.Fatalf("MaxBudget = %d, want 2000", report.MaxBudget)
	}
}

func TestPromptAssemblerBuildPromptShapesAndReports(t *testing.T) {
	buildLines := strings.Repeat("build line\n", 30)
	validateLines := strings.Repeat("validate line\n", 30)
	base := "## build\n" + buildLines + "\n## validate\n" + validateLines
	project := "project context line"
	instance := "instance details"
	fragment := strings.Repeat("fragment detail\n", 200)
	assembler := NewPromptAssembler(base, project, instance, fragment)
	assembler.MaxChars = 1200
	bead := BeadInfo{
		Title:     "cmd/main.go",
		Files:     []string{"cmd/main.go"},
		FileCount: 2,
	}
	output, report := assembler.AssembleWithReport("build", bead)
	if report == nil {
		t.Fatal("expected LastReport after Assemble")
	}
	if !strings.Contains(output, "=== BASE ===") {
		t.Fatal("prompt should include base layer marker")
	}
	if !strings.Contains(output, "=== PROJECT ===") {
		t.Fatal("prompt should include project layer marker")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatal("prompt should include instance layer marker")
	}
	if !strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatal("prompt should include fragment layer marker")
	}
	if !strings.Contains(output, "build line") {
		t.Fatal("build section should be present for build phase")
	}
	if strings.Contains(output, "validate line") {
		t.Fatal("validate section should be omitted when phase=build")
	}
	if report.FileCount != 2 {
		t.Fatalf("report.FileCount = %d, want 2", report.FileCount)
	}
	if report.AdjustedBudget != 600 {
		t.Fatalf("AdjustedBudget = %d, want 600", report.AdjustedBudget)
	}
	if !report.Trimmed {
		t.Fatal("expected report to indicate trimming occurred")
	}
	if report.TrimmedBytes <= 0 {
		t.Fatalf("TrimmedBytes = %d, want > 0", report.TrimmedBytes)
	}
	if report.ShapedSize != len(output) {
		t.Fatalf("ShapedSize = %d, want %d", report.ShapedSize, len(output))
	}
	if report.OriginalSize <= report.AdjustedBudget {
		t.Fatalf("OriginalSize = %d should be larger than AdjustedBudget %d", report.OriginalSize, report.AdjustedBudget)
	}
	if report.ShapedSize > report.AdjustedBudget {
		t.Fatalf("ShapedSize = %d should not exceed AdjustedBudget %d", report.ShapedSize, report.AdjustedBudget)
	}
}

func TestMaxCharsDefaultWhenZero(t *testing.T) {
	assembler := NewPromptAssembler("base", "", "", "")
	assembler.Assemble("", BeadInfo{FileCount: 10})
	report := assembler.LastReport()
	if report.MaxBudget != DefaultMaxChars {
		t.Fatalf("MaxBudget = %d, want %d (default)", report.MaxBudget, DefaultMaxChars)
	}
}

func TestPhaseCapTruncatesLongSection(t *testing.T) {
	// "build" phase cap is 12800
	section := strings.Repeat("a", 20000)
	base := "## build\n" + section
	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})
	// The BASE section includes the "=== BASE ===" marker
	// The phase-filtered content should be capped at 12800 chars.
	if strings.Count(output, "a") > 12800 {
		t.Fatalf("phase section should be capped at 12800 chars, got %d 'a' chars", strings.Count(output, "a"))
	}
}

func TestPhaseCapDoesNotTruncateShortSection(t *testing.T) {
	section := strings.Repeat("b", 100)
	base := "## red\n" + section
	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("red", BeadInfo{FileCount: 10})
	if !strings.Contains(output, section) {
		t.Fatal("short section should not be truncated")
	}
}

func TestPhaseCapNotAppliedForUnknownPhase(t *testing.T) {
	section := strings.Repeat("c", 20000)
	base := "## deploy\n" + section
	assembler := NewPromptAssembler(base, "", "", "")
	assembler.MaxChars = 200000
	output := assembler.Assemble("deploy", BeadInfo{FileCount: 10})
	if strings.Count(output, "c") != 20000 {
		t.Fatalf("unknown phase should not be capped, got %d 'c' chars", strings.Count(output, "c"))
	}
}

func TestPhaseCapKeepsPhaseSectionWhenPreambleLarge(t *testing.T) {
	cap := phaseMaxChars["build"]
	if cap == 0 {
		t.Fatal("phase cap missing for build phase")
	}

	preamble := strings.Repeat("x", cap+500)
	base := preamble + "\n\n## build <!-- phases: build -->\nbuild content\n\n## plan <!-- phases: plan -->\nplan content"
	assembler := NewPromptAssembler(base, "", "", "")
	filtered := assembler.loadBaseInstructions("build")

	if !strings.Contains(filtered, "build content") {
		t.Fatalf("build content missing from filtered base:\n%s", filtered)
	}
	if strings.Contains(filtered, "plan content") {
		t.Fatalf("plan content should not appear for build phase")
	}
	if len(filtered) > cap {
		t.Fatalf("filtered output length %d exceeds cap %d", len(filtered), cap)
	}
}

func TestShapeBudgetUTF8Safety(t *testing.T) {
	// 3-byte UTF-8 character: 世 (U+4E16)
	content := strings.Repeat("世", 1000) // 3000 bytes
	shaped, report := ShapeBudget(content, 1, 3000)
	// Budget is 50% = 1500 bytes, which cuts mid-rune at byte 1500.
	// After UTF-8 fixup, should be valid and shorter.
	if !isValidUTF8(shaped) {
		t.Fatal("shaped output contains invalid UTF-8")
	}
	if report.Trimmed != true {
		t.Fatal("expected Trimmed = true")
	}
	// Verify the result is a multiple of 3 bytes (each rune is 3 bytes)
	if len(shaped)%3 != 0 {
		t.Fatalf("shaped length %d is not a multiple of 3 (rune size)", len(shaped))
	}
}

func TestShapeBudgetUTF8SafetyNoCutNeeded(t *testing.T) {
	// All ASCII - no UTF-8 fixup needed
	content := strings.Repeat("x", 100)
	shaped, _ := ShapeBudget(content, 1, 100)
	if shaped != content[:50] {
		t.Fatalf("ASCII content should truncate cleanly to 50 bytes")
	}
}

func isValidUTF8(s string) bool {
	for len(s) > 0 {
		_, size := __utf8DecodeRuneInString(s)
		if size == 0 {
			return false
		}
		s = s[size:]
	}
	return true
}

// Thin wrapper to avoid importing utf8 in test; just use range.
func __utf8DecodeRuneInString(s string) (rune, int) {
	for i, r := range s {
		_ = i
		return r, len(string(r))
	}
	return 0, 0
}

func TestPhaseFilterExactMatch(t *testing.T) {
	base := "## builder\nbuilder instructions\n\n## build\nbuild instructions\n\n## validate\nvalidate instructions"
	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "build instructions") {
		t.Fatal("output should contain build instructions")
	}
	if strings.Contains(output, "builder instructions") {
		t.Fatal("output should NOT contain 'builder' instructions when phase is 'build'")
	}
	if strings.Contains(output, "validate instructions") {
		t.Fatal("output should NOT contain validate instructions")
	}
}

func TestPhaseFilterMatchesAnnotatedHeadings(t *testing.T) {
	base := "## Code Style <!-- phases: red, build, green -->\ncode style rules\n\n## Architecture <!-- phases: build, review -->\narchitecture rules\n\n## Decomposition <!-- phases: plan -->\ndecomposition rules\n\n## Retro <!-- phases: retro -->\nretro rules"

	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "code style rules") {
		t.Fatal("output should contain 'code style rules' (build is in its phases)")
	}
	if !strings.Contains(output, "architecture rules") {
		t.Fatal("output should contain 'architecture rules' (build is in its phases)")
	}
	if strings.Contains(output, "decomposition rules") {
		t.Fatal("output should NOT contain 'decomposition rules' (build not in plan-only section)")
	}
	if strings.Contains(output, "retro rules") {
		t.Fatal("output should NOT contain 'retro rules' (build not in retro-only section)")
	}
}

func TestPhaseFilterAnnotationMultipleSections(t *testing.T) {
	base := "## Safety <!-- phases: red, build, green, refactor, review -->\nsafety rules\n\n## Build Process <!-- phases: build -->\nbuild process rules\n\n## Test Quality <!-- phases: red, build, green, refactor, review -->\ntest quality rules"

	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "safety rules") {
		t.Fatal("output should contain safety rules")
	}
	if !strings.Contains(output, "build process rules") {
		t.Fatal("output should contain build process rules")
	}
	if !strings.Contains(output, "test quality rules") {
		t.Fatal("output should contain test quality rules")
	}
}

func TestPhaseFilterAnnotationExcludesNonMatchingPhase(t *testing.T) {
	base := "## Build Process <!-- phases: build -->\nbuild only\n\n## Decomposition <!-- phases: plan -->\nplan only"

	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("plan", BeadInfo{FileCount: 10})

	if strings.Contains(output, "build only") {
		t.Fatal("output should NOT contain build-only content when phase=plan")
	}
	if !strings.Contains(output, "plan only") {
		t.Fatal("output should contain plan-only content")
	}
}

func TestPhaseFilterNoAnnotationFallsBackToHeadingMatch(t *testing.T) {
	base := "## build\nbuild instructions\n\n## validate\nvalidate instructions"
	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "build instructions") {
		t.Fatal("output should contain build instructions (exact heading match)")
	}
	if strings.Contains(output, "validate instructions") {
		t.Fatal("output should NOT contain validate instructions")
	}
}

func TestPhaseFilterAnnotationReducesPromptSize(t *testing.T) {
	buildContent := strings.Repeat("b", 5000)
	planContent := strings.Repeat("p", 5000)
	retroContent := strings.Repeat("r", 5000)
	base := "## Code Style <!-- phases: build -->\n" + buildContent +
		"\n\n## Decomposition <!-- phases: plan -->\n" + planContent +
		"\n\n## Retro <!-- phases: retro -->\n" + retroContent

	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	baseContent := output[strings.Index(output, "=== BASE ==="):]
	if strings.Contains(baseContent, strings.Repeat("p", 100)) {
		t.Fatal("plan content should not appear in build phase")
	}
	if strings.Contains(baseContent, strings.Repeat("r", 100)) {
		t.Fatal("retro content should not appear in build phase")
	}
}

func TestPhaseFilterHeadingWithoutAnnotationIncludedAlways(t *testing.T) {
	base := "## General Rules\nalways included\n\n## Build Process <!-- phases: build -->\nbuild only"

	assembler := NewPromptAssembler(base, "", "", "")
	output := assembler.Assemble("plan", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "always included") {
		t.Fatal("sections without phase annotations should be included for all phases")
	}
	if strings.Contains(output, "build only") {
		t.Fatal("build-annotated section should NOT appear for plan phase")
	}
}

func TestProjectContextKeepsURLsAndSlashWords(t *testing.T) {
	project := "Use gRPC/HTTP for communication\nCheck https://example.com/docs\ninternal/runner/loop.go has the main loop\nUse input/output pattern"
	assembler := NewPromptAssembler("base", project, "", "")
	output := assembler.Assemble("", BeadInfo{Files: []string{"cmd/main.go"}})

	// Lines with URLs and slash-words (not file paths) should be kept
	if !strings.Contains(output, "gRPC/HTTP") {
		t.Fatal("gRPC/HTTP line should be kept (not a file path)")
	}
	if !strings.Contains(output, "https://example.com/docs") {
		t.Fatal("URL line should be kept (not a file path)")
	}
	if !strings.Contains(output, "input/output") {
		t.Fatal("input/output line should be kept (not a file path)")
	}
	// Line with internal/runner/loop.go should be filtered (not in bead files)
	if strings.Contains(output, "internal/runner/loop.go") {
		t.Fatal("internal/runner/loop.go should be filtered out (file path not in bead)")
	}
}

func TestProjectContextScopesArchitectureToBeadPaths(t *testing.T) {
	project := "# Project\n\n## Architecture\n- internal/target — keep this\n- internal/other — drop this"
	assembler := NewPromptAssembler("base", project, "", "")
	scoped := assembler.loadProjectContext(BeadInfo{
		Title: "internal/target",
		Files: []string{"cmd/main.go"},
	})

	if !strings.Contains(scoped, "internal/target") {
		t.Fatalf("expected relevant architecture bullet to remain:\n%s", scoped)
	}
	if strings.Contains(scoped, "internal/other") {
		t.Fatalf("unexpected architecture bullet for other package:\n%s", scoped)
	}
}
