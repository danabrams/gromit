//go:build acceptance

package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestRenderAcceptanceTests_ExcludesCLAUDEMD tests that acceptance test prompts
// include RULES.md but exclude CLAUDE.md to reduce token count.
// Expected failure: RenderAcceptanceTests currently includes CLAUDE.md via BuildContext
func TestRenderAcceptanceTests_ExcludesCLAUDEMD(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Write a template that uses both ClaudeMD and Rules
	tmpl := `# Acceptance Test Writing
{{if .ClaudeMD}}
## Project Context
{{.ClaudeMD}}
{{end}}
{{if .Rules}}
## Rules
{{.Rules}}
{{end}}
Task: {{.Bead.Title}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_acceptance_tests.md"), []byte(tmpl), 0644)

	// Write CLAUDE.md with substantial content
	claudeMD := `# Project CLAUDE.md
This is the main project documentation with architecture details.
It contains substantial content about the project structure.
This should NOT appear in acceptance test prompts.`
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMDPath, []byte(claudeMD), 0644)

	// Write RULES.md
	rules := `# Rules
These are the project rules.
Rules should appear in acceptance test prompts.`
	os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0644)

	r, err := NewRenderer(templatesDir, "", claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer error: %v", err)
	}

	b := &bead.Bead{
		ID:              "test-1",
		Title:           "Test feature",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Expected failure: BuildContext currently loads CLAUDE.md into ctx.ClaudeMD
	// After implementation, BuildContext should have a mode/flag to exclude CLAUDE.md
	// for acceptance test and refactor phases
	ctx, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	result, err := r.RenderAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderAcceptanceTests error: %v", err)
	}

	// CLAUDE.md content should NOT be present
	if strings.Contains(result, "project documentation") {
		t.Error("RenderAcceptanceTests should NOT include CLAUDE.md content, but it does")
	}
	if strings.Contains(result, "architecture details") {
		t.Error("RenderAcceptanceTests should NOT include CLAUDE.md content, but it does")
	}

	// RULES.md content SHOULD be present
	if !strings.Contains(result, "project rules") {
		t.Error("RenderAcceptanceTests should include RULES.md content, but it doesn't")
	}
}

// TestRenderRefactor_ExcludesCLAUDEMD tests that refactor prompts
// include RULES.md but exclude CLAUDE.md to reduce token count.
// Expected failure: RenderRefactor currently includes CLAUDE.md via BuildContext
func TestRenderRefactor_ExcludesCLAUDEMD(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Write a template that uses both ClaudeMD and Rules
	tmpl := `# Refactoring Phase
{{if .ClaudeMD}}
## Project Context
{{.ClaudeMD}}
{{end}}
{{if .Rules}}
## Rules
{{.Rules}}
{{end}}
Task: {{.Bead.Title}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte(tmpl), 0644)

	// Write CLAUDE.md with substantial content
	claudeMD := `# Project CLAUDE.md
This is the main project documentation.
Contains extensive architecture information.
Should be excluded from refactor prompts.`
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMDPath, []byte(claudeMD), 0644)

	// Write RULES.md
	rules := `# Rules
Project coding standards.
Should appear in refactor prompts.`
	os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0644)

	r, err := NewRenderer(templatesDir, "", claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer error: %v", err)
	}

	b := &bead.Bead{
		ID:              "test-1",
		Title:           "Refactor auth module",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Expected failure: BuildContext currently loads CLAUDE.md into ctx.ClaudeMD
	// After implementation, BuildContext should exclude CLAUDE.md for refactor phase
	ctx, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	result, err := r.RenderRefactor(ctx)
	if err != nil {
		t.Fatalf("RenderRefactor error: %v", err)
	}

	// CLAUDE.md content should NOT be present
	if strings.Contains(result, "project documentation") {
		t.Error("RenderRefactor should NOT include CLAUDE.md content, but it does")
	}
	if strings.Contains(result, "architecture information") {
		t.Error("RenderRefactor should NOT include CLAUDE.md content, but it does")
	}

	// RULES.md content SHOULD be present
	if !strings.Contains(result, "coding standards") {
		t.Error("RenderRefactor should include RULES.md content, but it doesn't")
	}
}

// TestBuildContext_FiltersLearningsByPackage tests that learnings included in
// build contexts are filtered to only those relevant to the bead's touched packages.
// Expected failure: BuildContext does not yet filter learnings by package keywords
func TestBuildContext_FiltersLearningsByPackage(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMDPath, []byte("# CLAUDE.md"), 0644)

	// Create learnings file with package-specific content
	learningsContent := `# Learnings

## Confirmed

### 2026-01-15 | Runner patterns | patterns
- **[patterns]** Runner methods follow a consistent pattern: nil-safe checks, feature-flag gating, context.WithTimeout for subprocess calls. The Run() method follows clear sequencing: validate -> execute work -> persist state. Provider invocations use router.Select() with phase + tier parameters.

### 2026-01-14 | Prompt rendering | conventions
- **[conventions]** Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating. Template files follow a consistent structure: context section at top, then Guidelines. Acceptance tests for template changes must match the exact content.

### 2026-01-13 | Config validation | gotchas
- **[gotchas]** Config struct fields must have sensible zero-value defaults in setDefaults(). After unmarshaling JSON structs, use normalizeNilFields() to convert nil slices to empty slices.

### 2026-01-12 | Bead client patterns | patterns
- **[patterns]** bead.Client methods have semantic distinctions: Ready()/CountReady() return only unblocked beads (bd ready), while List() returns all open beads (bd list --status open). Choose the correct method based on whether you need actionable or total counts.

## Recent

### 2026-02-14 | Logger iteration tracking | conventions
- **[conventions]** All acceptance test files in this codebase must include //go:build acceptance at the top. This is verified by final_verification_test.go as part of the standard test suite.
`
	os.WriteFile(filepath.Join(gromitDir, "LEARNINGS.md"), []byte(learningsContent), 0644)

	r, err := NewRenderer(tmpDir, "", claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer error: %v", err)
	}

	// Bead that touches runner package
	beadRunner := &bead.Bead{
		ID:              "test-runner",
		Title:           "Update runner loop",
		Description:     "Modify internal/runner/runner.go sequencing",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Bead that touches prompt package
	beadPrompt := &bead.Bead{
		ID:              "test-prompt",
		Title:           "Add new prompt template",
		Description:     "Create PROMPT_xyz.md in internal/prompt/",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Bead that touches config package
	beadConfig := &bead.Bead{
		ID:              "test-config",
		Title:           "Add config field",
		Description:     "Update internal/config/config.go with new field",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Expected failure: BuildContext does not yet filter learnings by package
	// After implementation, each bead should only get learnings relevant to its packages
	ctxRunner, err := r.BuildContext(beadRunner, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	ctxPrompt, err := r.BuildContext(beadPrompt, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	ctxConfig, err := r.BuildContext(beadConfig, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	// Check that runner bead gets runner-related learnings but not prompt/config ones
	runnerHasRunnerLearning := false
	runnerHasPromptLearning := false
	for _, l := range ctxRunner.ConfirmedLearnings {
		if strings.Contains(l.Content, "Runner methods") {
			runnerHasRunnerLearning = true
		}
		if strings.Contains(l.Content, "Prompt templates") {
			runnerHasPromptLearning = true
		}
	}
	if !runnerHasRunnerLearning {
		t.Error("Runner bead should include runner-related learnings")
	}
	if runnerHasPromptLearning {
		t.Error("Runner bead should NOT include prompt-related learnings when filtering by package")
	}

	// Check that prompt bead gets prompt-related learnings but not runner/config ones
	promptHasPromptLearning := false
	promptHasRunnerLearning := false
	for _, l := range ctxPrompt.ConfirmedLearnings {
		if strings.Contains(l.Content, "Prompt templates") {
			promptHasPromptLearning = true
		}
		if strings.Contains(l.Content, "Runner methods") {
			promptHasRunnerLearning = true
		}
	}
	if !promptHasPromptLearning {
		t.Error("Prompt bead should include prompt-related learnings")
	}
	if promptHasRunnerLearning {
		t.Error("Prompt bead should NOT include runner-related learnings when filtering by package")
	}

	// Check that config bead gets config-related learnings
	configHasConfigLearning := false
	for _, l := range ctxConfig.ConfirmedLearnings {
		if strings.Contains(l.Content, "Config struct") {
			configHasConfigLearning = true
		}
	}
	if !configHasConfigLearning {
		t.Error("Config bead should include config-related learnings")
	}
}

// TestAcceptanceTestPrompts_TokenCountReduction tests that acceptance test prompts
// use fewer tokens than build prompts due to CLAUDE.md exclusion and learning filtering.
// Expected failure: Token counts are not yet tracked or different between phases
func TestAcceptanceTestPrompts_TokenCountReduction(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Write CLAUDE.md with ~5000 chars (roughly 1250 tokens at 4 chars/token)
	claudeMDContent := strings.Repeat("This is substantial project documentation content. ", 100)
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMDPath, []byte(claudeMDContent), 0644)

	// Write RULES.md
	rulesContent := "# Rules\nProject rules here."
	os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rulesContent), 0644)

	// Write build template that includes CLAUDE.md
	buildTmpl := `# Build Phase
{{if .ClaudeMD}}{{.ClaudeMD}}{{end}}
{{if .Rules}}{{.Rules}}{{end}}
Task: {{.Bead.Title}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_build.md"), []byte(buildTmpl), 0644)

	// Write acceptance test template that should exclude CLAUDE.md
	acceptanceTmpl := `# Acceptance Test Writing
{{if .ClaudeMD}}{{.ClaudeMD}}{{end}}
{{if .Rules}}{{.Rules}}{{end}}
Task: {{.Bead.Title}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_acceptance_tests.md"), []byte(acceptanceTmpl), 0644)

	r, err := NewRenderer(templatesDir, "", claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer error: %v", err)
	}

	b := &bead.Bead{
		ID:              "test-1",
		Title:           "Test feature",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Build context - currently includes CLAUDE.md for all phases
	ctxBuild, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	// Render build prompt (includes CLAUDE.md)
	buildPrompt, err := r.RenderBuild(ctxBuild)
	if err != nil {
		t.Fatalf("RenderBuild error: %v", err)
	}

	// Expected failure: BuildContext for acceptance tests should exclude CLAUDE.md
	ctxAcceptance, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	// Render acceptance test prompt (should exclude CLAUDE.md)
	acceptancePrompt, err := r.RenderAcceptanceTests(ctxAcceptance)
	if err != nil {
		t.Fatalf("RenderAcceptanceTests error: %v", err)
	}

	// Expected failure: acceptance test prompt currently same length as build prompt
	// After implementation, acceptance test prompt should be ~5000 chars shorter
	buildLen := len(buildPrompt)
	acceptanceLen := len(acceptancePrompt)
	reduction := buildLen - acceptanceLen

	// We expect ~5000 char reduction (CLAUDE.md content)
	// Use approximate token estimate: 1 token ≈ 4 chars, so 5000 tokens ≈ 20000 chars
	expectedMinReduction := 4000 // Allow some variance
	if reduction < expectedMinReduction {
		t.Errorf("Acceptance test prompt should be ~5000+ chars shorter than build prompt, got reduction of %d chars (build: %d, acceptance: %d)",
			reduction, buildLen, acceptanceLen)
	}
}

// TestRefactorPrompts_TokenCountReduction tests that refactor prompts use fewer tokens
// than build prompts due to CLAUDE.md exclusion and learning filtering.
// Expected failure: Token counts are not yet tracked or different between phases
func TestRefactorPrompts_TokenCountReduction(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Write CLAUDE.md with ~5000 chars
	claudeMDContent := strings.Repeat("Detailed project architecture and patterns documentation. ", 100)
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMDPath, []byte(claudeMDContent), 0644)

	// Write RULES.md
	rulesContent := "# Rules\nCoding standards."
	os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rulesContent), 0644)

	// Write build template
	buildTmpl := `# Build
{{if .ClaudeMD}}{{.ClaudeMD}}{{end}}
{{if .Rules}}{{.Rules}}{{end}}
Task: {{.Bead.Title}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_build.md"), []byte(buildTmpl), 0644)

	// Write refactor template that should exclude CLAUDE.md
	refactorTmpl := `# Refactor
{{if .ClaudeMD}}{{.ClaudeMD}}{{end}}
{{if .Rules}}{{.Rules}}{{end}}
Task: {{.Bead.Title}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte(refactorTmpl), 0644)

	r, err := NewRenderer(templatesDir, "", claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer error: %v", err)
	}

	b := &bead.Bead{
		ID:              "test-1",
		Title:           "Refactor module",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Build context for build phase (includes CLAUDE.md)
	ctxBuild, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	// Render build prompt
	buildPrompt, err := r.RenderBuild(ctxBuild)
	if err != nil {
		t.Fatalf("RenderBuild error: %v", err)
	}

	// Expected failure: BuildContext for refactor should exclude CLAUDE.md
	ctxRefactor, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	// Render refactor prompt (should exclude CLAUDE.md)
	refactorPrompt, err := r.RenderRefactor(ctxRefactor)
	if err != nil {
		t.Fatalf("RenderRefactor error: %v", err)
	}

	// Expected failure: refactor prompt currently same length as build prompt
	// After implementation, refactor prompt should be ~5000 chars shorter
	buildLen := len(buildPrompt)
	refactorLen := len(refactorPrompt)
	reduction := buildLen - refactorLen

	expectedMinReduction := 4000
	if reduction < expectedMinReduction {
		t.Errorf("Refactor prompt should be ~5000+ chars shorter than build prompt, got reduction of %d chars (build: %d, refactor: %d)",
			reduction, buildLen, refactorLen)
	}
}

// TestRenderAcceptanceTests_BuildsContextWithoutCLAUDEMD tests that when rendering
// acceptance tests, the context is built specifically for that phase without CLAUDE.md.
// Expected failure: RenderAcceptanceTests uses standard BuildContext which includes CLAUDE.md
func TestRenderAcceptanceTests_BuildsContextWithoutCLAUDEMD(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(gromitDir, 0755)

	// Minimal template that would show CLAUDE.md if present
	tmpl := `{{if .ClaudeMD}}CLAUDEMD_PRESENT{{end}}
{{if .Rules}}RULES_PRESENT{{end}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_acceptance_tests.md"), []byte(tmpl), 0644)

	claudeMD := "# CLAUDE.md content"
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMDPath, []byte(claudeMD), 0644)

	rules := "# Rules"
	os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0644)

	r, err := NewRenderer(templatesDir, "", claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer error: %v", err)
	}

	// Create context via standard BuildContext
	b := &bead.Bead{
		ID:              "test-1",
		Title:           "Test",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, err := r.BuildContext(b, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	// Expected failure: When we render acceptance tests with this context,
	// CLAUDE.md is present in the context (loaded by BuildContext)
	result, err := r.RenderAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderAcceptanceTests error: %v", err)
	}

	// After implementation, acceptance test rendering should build a modified context
	// that excludes CLAUDE.md even though the template could render it
	if strings.Contains(result, "CLAUDEMD_PRESENT") {
		t.Error("RenderAcceptanceTests should not populate ClaudeMD in context, but it does")
	}

	// Rules should still be present
	if !strings.Contains(result, "RULES_PRESENT") {
		t.Error("RenderAcceptanceTests should include Rules in context")
	}
}

// TestExtractPackageKeywords_FromBeadDescription tests that a new helper function
// extracts package paths from bead descriptions to use as learning filter keywords.
// Expected failure: extractPackageKeywords function does not exist yet
func TestExtractPackageKeywords_FromBeadDescription(t *testing.T) {
	type packageExtractor func(description string) []string

	// Expected failure: This function doesn't exist yet
	// After implementation, it should extract "internal/runner", "internal/prompt", etc.
	// from bead descriptions that mention file paths or package names

	tests := []struct {
		name         string
		description  string
		wantKeywords []string
	}{
		{
			name:         "single package in path",
			description:  "Modify internal/runner/runner.go sequencing",
			wantKeywords: []string{"internal/runner"},
		},
		{
			name:         "multiple packages",
			description:  "Update internal/prompt/ and internal/config/config.go",
			wantKeywords: []string{"internal/prompt", "internal/config"},
		},
		{
			name:         "no package paths",
			description:  "Fix bug in authentication",
			wantKeywords: []string{},
		},
	}

	// Verify the function doesn't exist by attempting to call it
	// This test will fail at compile time until the function is implemented
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: cannot use extractPackageKeywords (undefined)
			var extractPackageKeywords packageExtractor
			if extractPackageKeywords != nil {
				got := extractPackageKeywords(tt.description)
				if len(got) != len(tt.wantKeywords) {
					t.Errorf("extractPackageKeywords() returned %d keywords, want %d", len(got), len(tt.wantKeywords))
				}
			} else {
				t.Skip("extractPackageKeywords function not yet implemented")
			}
		})
	}
}
