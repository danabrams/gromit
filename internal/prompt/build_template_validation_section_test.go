package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// setupRealTemplateRenderer creates a Renderer pointing at the real project templates.
// Returns nil and skips the test if the templates directory doesn't exist.
func setupRealTemplateRenderer(t *testing.T) *Renderer {
	t.Helper()
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	if _, err := os.Stat(filepath.Join(templatesDir, "PROMPT_build.md")); os.IsNotExist(err) {
		t.Skipf("skipping: real templates not found at %s", templatesDir)
	}
	return &Renderer{templatesDir: templatesDir}
}

func testBead() *bead.Bead {
	return &bead.Bead{
		ID:              "test-1",
		Title:           "Implement feature X",
		Priority:        1,
		Description:     "Add new functionality",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}
}

// TestBuildTemplatesRenderValidationFailuresSection verifies that all three build
// templates render a "Recent Validation Issues" section when RecentValidationFailures
// is populated, and omit it when empty.
func TestBuildTemplatesRenderValidationFailuresSection(t *testing.T) {
	r := setupRealTemplateRenderer(t)

	failures := []string{
		"--- FAIL: TestAuth (0.01s)\nFAIL\tgithub.com/example/auth",
		"./handler.go:42:5: unused variable 'ctx'",
		"--- FAIL: TestDB (0.03s)\nFAIL\tgithub.com/example/db",
	}

	type renderFunc func(*Context) (string, error)

	templates := []struct {
		name   string
		render renderFunc
	}{
		{"PROMPT_build.md", r.RenderBuild},
		{"PROMPT_atdd_build.md", r.RenderATDDBuild},
		{"PROMPT_tdd_build.md", r.RenderTDDBuild},
	}

	for _, tmpl := range templates {
		t.Run(tmpl.name+" with failures present", func(t *testing.T) {
			ctx := &Context{
				Bead:                     testBead(),
				Model:                    "sonnet",
				Iteration:                1,
				RecentValidationFailures: failures,
			}

			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			if !strings.Contains(result, "Recent Validation Issues") {
				t.Errorf("%s: expected 'Recent Validation Issues' section header when failures are present, but it was not found in rendered output", tmpl.name)
			}

			// Each failure summary should appear in the output
			for _, f := range failures {
				if !strings.Contains(result, f) {
					t.Errorf("%s: expected failure summary %q to appear in rendered output", tmpl.name, f)
				}
			}
		})

		t.Run(tmpl.name+" with no failures", func(t *testing.T) {
			ctx := &Context{
				Bead:                     testBead(),
				Model:                    "sonnet",
				Iteration:                1,
				RecentValidationFailures: []string{},
			}

			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			if strings.Contains(result, "Recent Validation Issues") {
				t.Errorf("%s: 'Recent Validation Issues' section should be omitted when no failures exist", tmpl.name)
			}
		})
	}
}

// TestBuildTemplatesValidationFailuresSectionPlacement verifies that the
// "Recent Validation Issues" section is placed between the Learnings sections
// and the Project Context section in all three build templates.
func TestBuildTemplatesValidationFailuresSectionPlacement(t *testing.T) {
	r := setupRealTemplateRenderer(t)

	type renderFunc func(*Context) (string, error)

	templates := []struct {
		name   string
		render renderFunc
	}{
		{"PROMPT_build.md", r.RenderBuild},
		{"PROMPT_atdd_build.md", r.RenderATDDBuild},
		{"PROMPT_tdd_build.md", r.RenderTDDBuild},
	}

	for _, tmpl := range templates {
		t.Run(tmpl.name, func(t *testing.T) {
			ctx := &Context{
				Bead:      testBead(),
				Model:     "sonnet",
				Iteration: 1,
				RecentValidationFailures: []string{
					"--- FAIL: TestSomething (0.01s)",
				},
			}

			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			validationIdx := strings.Index(result, "Recent Validation Issues")
			if validationIdx == -1 {
				t.Fatalf("%s: 'Recent Validation Issues' section not found in output", tmpl.name)
			}

			projectCtxIdx := strings.Index(result, "## Project Context")
			if projectCtxIdx == -1 {
				t.Fatalf("%s: '## Project Context' section not found in output", tmpl.name)
			}

			if validationIdx >= projectCtxIdx {
				t.Errorf("%s: 'Recent Validation Issues' (at pos %d) must appear before '## Project Context' (at pos %d)",
					tmpl.name, validationIdx, projectCtxIdx)
			}
		})
	}
}

func TestBuildTemplatesRenderCommonReviewFindingsSection(t *testing.T) {
	r := setupRealTemplateRenderer(t)

	type renderFunc func(*Context) (string, error)
	templates := []struct {
		name   string
		render renderFunc
	}{
		{"PROMPT_build.md", r.RenderBuild},
		{"PROMPT_atdd_build.md", r.RenderATDDBuild},
		{"PROMPT_tdd_build.md", r.RenderTDDBuild},
	}

	findings := []string{"error_handling", "nil_checks"}
	for _, tmpl := range templates {
		t.Run(tmpl.name+" with findings", func(t *testing.T) {
			ctx := &Context{
				Bead:                 testBead(),
				Model:                "sonnet",
				Iteration:            1,
				CommonReviewFindings: findings,
			}
			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if !strings.Contains(result, "Common Review Findings To Avoid") {
				t.Fatalf("%s: expected common findings section", tmpl.name)
			}
			for _, finding := range findings {
				if !strings.Contains(result, finding) {
					t.Fatalf("%s: missing finding %q", tmpl.name, finding)
				}
			}
		})

		t.Run(tmpl.name+" without findings", func(t *testing.T) {
			ctx := &Context{
				Bead:                 testBead(),
				Model:                "sonnet",
				Iteration:            1,
				CommonReviewFindings: []string{},
			}
			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if strings.Contains(result, "Common Review Findings To Avoid") {
				t.Fatalf("%s: section should be omitted when no findings", tmpl.name)
			}
		})
	}
}

// TestBuildTemplatesScopedTestCommand verifies that when ScopedTestCommand is set
// on the Context, all three build templates render that exact command in the
// Instructions section instead of the generic "./..." form.
func TestBuildTemplatesScopedTestCommand(t *testing.T) {
	r := setupRealTemplateRenderer(t)

	type renderFunc func(*Context) (string, error)

	templates := []struct {
		name   string
		render renderFunc
	}{
		{"PROMPT_build.md", r.RenderBuild},
		{"PROMPT_atdd_build.md", r.RenderATDDBuild},
		{"PROMPT_tdd_build.md", r.RenderTDDBuild},
	}

	scopedCmd := "go test ./internal/runner/... ./internal/config/..."

	for _, tmpl := range templates {
		t.Run(tmpl.name+" with scoped command", func(t *testing.T) {
			ctx := &Context{
				Bead:              testBead(),
				Model:             "sonnet",
				Iteration:         1,
				ScopedTestCommand: scopedCmd,
			}

			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			if !strings.Contains(result, scopedCmd) {
				t.Errorf("%s: expected scoped test command %q to appear in rendered output when ScopedTestCommand is set", tmpl.name, scopedCmd)
			}
		})

		t.Run(tmpl.name+" without scoped command falls back to generic", func(t *testing.T) {
			ctx := &Context{
				Bead:      testBead(),
				Model:     "sonnet",
				Iteration: 1,
			}

			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			// Without ScopedTestCommand, the template should still mention go test
			if !strings.Contains(result, "go test") {
				t.Errorf("%s: expected 'go test' to appear in rendered output when ScopedTestCommand is empty", tmpl.name)
			}
		})
	}
}

// TestBuildTemplatesSelfCheckGuidance verifies that all three build templates
// include self-check guidance in their Instructions section, instructing the
// agent to run validation commands before completing.
func TestBuildTemplatesSelfCheckGuidance(t *testing.T) {
	r := setupRealTemplateRenderer(t)

	type renderFunc func(*Context) (string, error)

	templates := []struct {
		name   string
		render renderFunc
	}{
		{"PROMPT_build.md", r.RenderBuild},
		{"PROMPT_atdd_build.md", r.RenderATDDBuild},
		{"PROMPT_tdd_build.md", r.RenderTDDBuild},
	}

	for _, tmpl := range templates {
		t.Run(tmpl.name, func(t *testing.T) {
			ctx := &Context{
				Bead:                     testBead(),
				Model:                    "sonnet",
				Iteration:                1,
				RecentValidationFailures: []string{},
			}

			result, err := tmpl.render(ctx)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			// The Instructions section must exist
			instructionsIdx := strings.Index(result, "## Instructions")
			if instructionsIdx == -1 {
				t.Fatalf("%s: '## Instructions' section not found", tmpl.name)
			}

			// Extract the Instructions section content (from header to next ## or end)
			afterInstructions := result[instructionsIdx:]
			nextSection := strings.Index(afterInstructions[len("## Instructions"):], "\n## ")
			var instructionsContent string
			if nextSection != -1 {
				instructionsContent = afterInstructions[:len("## Instructions")+nextSection]
			} else {
				instructionsContent = afterInstructions
			}

			// Must mention running validation/test commands before completing
			if !strings.Contains(instructionsContent, "go test") {
				t.Errorf("%s: Instructions section should contain self-check guidance mentioning 'go test'", tmpl.name)
			}
			if !strings.Contains(instructionsContent, "go vet") {
				t.Errorf("%s: Instructions section should contain self-check guidance mentioning 'go vet'", tmpl.name)
			}

			// Must indicate this should be done before completing/committing
			hasBeforeCompleting := strings.Contains(instructionsContent, "before completing") ||
				strings.Contains(instructionsContent, "before committing") ||
				strings.Contains(instructionsContent, "before finishing") ||
				strings.Contains(instructionsContent, "Fix any failures")
			if !hasBeforeCompleting {
				t.Errorf("%s: Instructions section should indicate validation should run before completing/committing", tmpl.name)
			}
		})
	}
}
