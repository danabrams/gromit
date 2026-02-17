package prompt

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

func TestValidateSpecName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "auth", false},
		{"valid hyphenated name", "user-auth", false},
		{"valid underscore name", "user_auth", false},
		{"empty name", "", true},
		{"dot-dot traversal", "../../etc/passwd", true},
		{"single dot-dot", "..", true},
		{"dot-dot in middle", "foo/../bar", true},
		{"forward slash", "foo/bar", true},
		{"backslash", "foo\\bar", true},
		{"absolute path attempt", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpecName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpecName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestLoadSpec_PathTraversal(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	os.MkdirAll(specsDir, 0755)

	// Create a valid spec file
	os.WriteFile(filepath.Join(specsDir, "valid.md"), []byte("# Valid Spec"), 0644)

	// Create a secret file outside specs dir
	os.WriteFile(filepath.Join(tmpDir, "secret.md"), []byte("SECRET DATA"), 0644)

	r := &Renderer{specsDir: specsDir}

	// Valid spec should load fine
	content, err := r.LoadSpec("valid")
	if err != nil {
		t.Fatalf("LoadSpec(valid) unexpected error: %v", err)
	}
	if content != "# Valid Spec" {
		t.Errorf("LoadSpec(valid) = %q, want %q", content, "# Valid Spec")
	}

	// Path traversal attempts should be rejected
	traversalNames := []string{
		"../secret",
		"../../etc/passwd",
		"..",
		"foo/../../../secret",
		"foo/bar",
		"foo\\bar",
	}
	for _, name := range traversalNames {
		content, err := r.LoadSpec(name)
		if err == nil {
			t.Errorf("LoadSpec(%q) should have returned error, got content: %q", name, content)
		}
	}

	// Missing spec should return empty string, no error
	content, err = r.LoadSpec("nonexistent")
	if err != nil {
		t.Errorf("LoadSpec(nonexistent) unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("LoadSpec(nonexistent) = %q, want empty", content)
	}
}

func TestGetLearningsFileNilReceiver(t *testing.T) {
	var r *Renderer
	result := r.GetLearningsFile()
	if result != nil {
		t.Errorf("expected nil for nil Renderer, got %v", result)
	}
}

func TestBuildContextNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.BuildContext(nil, nil, 1, "sonnet")
	if err == nil {
		t.Error("expected error for nil renderer")
	}
}

func TestBuildContextNilBead(t *testing.T) {
	r := &Renderer{}
	_, err := r.BuildContext(nil, nil, 1, "sonnet")
	if err == nil {
		t.Error("expected error for nil bead")
	}
}

func TestRenderNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderBuild(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderBuild")
	}
	_, err = r.RenderAnalyze(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderAnalyze")
	}
	_, err = r.RenderAcceptanceTests(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderAcceptanceTests")
	}
	_, err = r.RenderATDDBuild(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderATDDBuild")
	}
	_, err = r.RenderRefactor(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderRefactor")
	}
	_, err = r.RenderTDDBuild(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderTDDBuild")
	}
}

func TestLoadClaudeMDNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadClaudeMD()
	if err == nil {
		t.Error("expected error for nil renderer in LoadClaudeMD")
	}
}

func TestLoadRulesNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadRules()
	if err == nil {
		t.Error("expected error for nil renderer in LoadRules")
	}
}

func TestLoadSpecNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadSpec("test")
	if err == nil {
		t.Error("expected error for nil renderer in LoadSpec")
	}
}

func TestRenderDecomposeNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderDecompose(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderDecompose")
	}
}

func TestRenderScopeNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderScope(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderScope")
	}
}

func TestRenderPrecheckNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderPrecheck(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderPrecheck")
	}
}

func TestRenderPrecheck(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := tmpDir
	os.MkdirAll(templatesDir, 0755)

	// Write a precheck template that uses PrecheckContext fields
	tmpl := `Pre-check: {{.Bead.Title}}
ID: {{.Bead.ID}}
Priority: P{{.Bead.Priority}}
{{if .Bead.Labels}}Labels: {{join .Bead.Labels ", "}}{{end}}
{{if .Bead.Description}}Description: {{.Bead.Description}}{{end}}
{{if .ParentBead}}Parent: {{.ParentBead.Title}}{{end}}`
	os.WriteFile(tmpDir+"/PROMPT_precheck.md", []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	tests := []struct {
		name    string
		ctx     *PrecheckContext
		wantStr []string // Expected substrings in output
	}{
		{
			name: "with populated context and no parent",
			ctx: &PrecheckContext{
				Bead: &bead.Bead{
					ID:              "test-123",
					Title:           "Test acceptance criteria",
					Priority:        2,
					Description:     "Check if criteria are met",
					Labels:          []string{},
					ExpectedOutputs: []string{},
				},
				ParentBead: nil,
			},
			wantStr: []string{
				"Test acceptance criteria",
				"test-123",
				"P2",
				"Check if criteria are met",
			},
		},
		{
			name: "with parent bead and labels",
			ctx: &PrecheckContext{
				Bead: &bead.Bead{
					ID:              "child-456",
					Title:           "Child task",
					Priority:        1,
					Description:     "Part of larger work",
					Labels:          []string{"from-review", "test-gap"},
					ExpectedOutputs: []string{},
				},
				ParentBead: &bead.Bead{
					ID:              "parent-123",
					Title:           "Parent epic",
					Priority:        0,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				},
			},
			wantStr: []string{
				"Child task",
				"child-456",
				"P1",
				"Part of larger work",
				"from-review, test-gap",
				"Parent epic",
			},
		},
		{
			name: "with empty labels slice",
			ctx: &PrecheckContext{
				Bead: &bead.Bead{
					ID:              "test-789",
					Title:           "Simple task",
					Priority:        3,
					Description:     "No labels",
					Labels:          []string{},
					ExpectedOutputs: []string{},
				},
				ParentBead: nil,
			},
			wantStr: []string{
				"Simple task",
				"test-789",
				"P3",
				"No labels",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.RenderPrecheck(tt.ctx)
			if err != nil {
				t.Fatalf("RenderPrecheck() error = %v", err)
			}
			if result == "" {
				t.Error("expected non-empty output")
			}
			for _, want := range tt.wantStr {
				if !strings.Contains(result, want) {
					t.Errorf("RenderPrecheck() missing expected string %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

func TestParseScopeEstimate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		checks  func(*ScopeEstimate) bool
	}{
		{
			name: "valid scope estimate",
			input: `{
  "complexity": "medium",
  "estimated_iterations": 2,
  "rationale": "Changes to 3-5 files with moderate testing",
  "can_complete_in_single_iteration": false,
  "blockers": ["Need to refactor auth system", "Database schema changes required"]
}`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "medium" &&
					est.EstimatedIterations == 2 &&
					est.Rationale == "Changes to 3-5 files with moderate testing" &&
					!est.CanCompleteInSingleIteration &&
					len(est.Blockers) == 2
			},
		},
		{
			name: "low complexity single iteration",
			input: `{
  "complexity": "low",
  "estimated_iterations": 1,
  "rationale": "Straightforward changes",
  "can_complete_in_single_iteration": true,
  "blockers": []
}`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "low" &&
					est.EstimatedIterations == 1 &&
					est.CanCompleteInSingleIteration &&
					len(est.Blockers) == 0
			},
		},
		{
			name: "high complexity with multiple blockers",
			input: `{
  "complexity": "high",
  "estimated_iterations": 4,
  "rationale": "Complex architecture changes across multiple systems",
  "can_complete_in_single_iteration": false,
  "blockers": ["Unclear requirements", "Cross-system dependencies", "Performance implications"]
}`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "high" &&
					est.EstimatedIterations == 4 &&
					len(est.Blockers) == 3
			},
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no JSON in output",
			input:   "This is just plain text with no JSON",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `{ "complexity": "low", invalid json }`,
			wantErr: true,
		},
		{
			name: "JSON with surrounding text",
			input: `Here is the scope estimate:
{
  "complexity": "low",
  "estimated_iterations": 1,
  "rationale": "Simple task",
  "can_complete_in_single_iteration": true,
  "blockers": []
}
Additional explanation follows...`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "low" &&
					est.EstimatedIterations == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est, err := ParseScopeEstimate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScopeEstimate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checks != nil {
				if !tt.checks(est) {
					t.Errorf("ParseScopeEstimate() returned unexpected values: %+v", est)
				}
			}
		})
	}
}

func TestContextNormalizeNilFields(t *testing.T) {
	// Test that nil learning slices are normalized to empty slices
	ctx := &Context{
		Iteration: 1,
		Model:     "sonnet",
	}
	if ctx.ConfirmedLearnings != nil {
		t.Error("expected ConfirmedLearnings to start as nil")
	}
	if ctx.RecentLearnings != nil {
		t.Error("expected RecentLearnings to start as nil")
	}

	ctx.normalizeNilFields()
	if ctx.ConfirmedLearnings == nil {
		t.Error("expected ConfirmedLearnings to be non-nil after normalization")
	}
	if ctx.RecentLearnings == nil {
		t.Error("expected RecentLearnings to be non-nil after normalization")
	}
	if len(ctx.ConfirmedLearnings) != 0 {
		t.Errorf("expected empty ConfirmedLearnings, got %d items", len(ctx.ConfirmedLearnings))
	}
	if len(ctx.RecentLearnings) != 0 {
		t.Errorf("expected empty RecentLearnings, got %d items", len(ctx.RecentLearnings))
	}
}

func TestContextNormalizeNilFieldsNilReceiver(t *testing.T) {
	var ctx *Context
	ctx.normalizeNilFields() // Should not panic
}

func TestContextNormalizeNilFieldsPreservesExisting(t *testing.T) {
	ctx := &Context{
		ConfirmedLearnings: []learnings.Learning{{Content: "a"}},
		RecentLearnings:    []learnings.Learning{{Content: "b"}, {Content: "c"}},
	}

	ctx.normalizeNilFields()
	if len(ctx.ConfirmedLearnings) != 1 {
		t.Errorf("expected 1 confirmed learning preserved, got %d", len(ctx.ConfirmedLearnings))
	}
	if len(ctx.RecentLearnings) != 2 {
		t.Errorf("expected 2 recent learnings preserved, got %d", len(ctx.RecentLearnings))
	}
}

func TestScopeEstimateNormalizeNilFields(t *testing.T) {
	tests := []struct {
		name     string
		estimate *ScopeEstimate
	}{
		{
			name:     "nil Blockers",
			estimate: &ScopeEstimate{Complexity: "low"},
		},
		{
			name:     "already non-nil",
			estimate: &ScopeEstimate{Complexity: "high", Blockers: []string{"blocker1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.estimate.normalizeNilFields()
			if tt.estimate.Blockers == nil {
				t.Error("Blockers should not be nil after normalization")
			}
		})
	}
}

func TestScopeEstimateNormalizeNilFieldsOnNilEstimate(t *testing.T) {
	var s *ScopeEstimate
	s.normalizeNilFields() // Should not panic
}

func TestParseScopeEstimateNormalizesNilBlockers(t *testing.T) {
	// JSON where blockers field is missing
	input := `{
		"complexity": "low",
		"estimated_iterations": 1,
		"rationale": "Simple task",
		"can_complete_in_single_iteration": true
	}`

	est, err := ParseScopeEstimate(input)
	if err != nil {
		t.Fatalf("ParseScopeEstimate() error = %v", err)
	}
	if est.Blockers == nil {
		t.Error("Blockers should not be nil after ParseScopeEstimate (missing field)")
	}
}

func TestParseScopeEstimateNormalizesExplicitNullBlockers(t *testing.T) {
	input := `{
		"complexity": "medium",
		"estimated_iterations": 2,
		"rationale": "Moderate task",
		"can_complete_in_single_iteration": false,
		"blockers": null
	}`

	est, err := ParseScopeEstimate(input)
	if err != nil {
		t.Fatalf("ParseScopeEstimate() error = %v", err)
	}
	if est.Blockers == nil {
		t.Error("Blockers should not be nil after ParseScopeEstimate (was JSON null)")
	}
}

func TestRenderReviewNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderReview(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderReview")
	}
}

func TestRenderThoroughReviewNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderThoroughReview(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderThoroughReview")
	}
}

func TestRenderReview(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	// Write a simple review template
	tmpl := `Review for: {{.Bead.Title}}
Diff: {{.Diff}}
Model: {{.Model}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_review.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &ReviewContext{
		Bead: &bead.Bead{
			ID:              "test-1",
			Title:           "Test bead",
			Description:     "Test description",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
		Diff:  "diff --git a/file.go\n+added line",
		Model: "sonnet",
	}

	result, err := r.RenderReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Test bead") {
		t.Error("expected bead title in output")
	}
	if !strings.Contains(result, "added line") {
		t.Error("expected diff in output")
	}
}

func TestRenderThoroughReview(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Thorough review
Beads completed: {{len .CompletedBeads}}
Diff: {{.Diff}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_thorough_review.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &ThoroughReviewContext{
		Diff: "full diff here",
		CompletedBeads: []CompletedBeadSummary{
			{ID: "b-1", Title: "First task"},
			{ID: "b-2", Title: "Second task"},
		},
	}

	result, err := r.RenderThoroughReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Beads completed: 2") {
		t.Error("expected 2 completed beads")
	}
}

func TestRenderAcceptanceTests(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Write acceptance tests for: {{.Bead.Title}}
Description: {{.Bead.Description}}
Model: {{.Model}}
Iteration: {{.Iteration}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_acceptance_tests.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &Context{
		Bead: &bead.Bead{
			ID:              "test-1",
			Title:           "Test feature",
			Description:     "Implement test feature",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
		Model:     "sonnet",
		Iteration: 1,
	}

	result, err := r.RenderAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("RenderAcceptanceTests() error = %v", err)
	}
	if result == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(result, "Test feature") {
		t.Error("expected bead title in output")
	}
	if !strings.Contains(result, "Implement test feature") {
		t.Error("expected bead description in output")
	}
	if !strings.Contains(result, "sonnet") {
		t.Error("expected model in output")
	}
}

func TestRenderSpecAcceptance(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Spec Acceptance\nRules: {{.Rules}}\nSpec: {{.Spec}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_spec_acceptance.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &SpecAcceptanceContext{
		Spec:  "# Spec\nDo the thing",
		Rules: "Always be explicit",
	}

	result, err := r.RenderSpecAcceptance(ctx)
	if err != nil {
		t.Fatalf("RenderSpecAcceptance() error = %v", err)
	}
	if !strings.Contains(result, "Always be explicit") {
		t.Error("expected rules in output")
	}
	if !strings.Contains(result, "# Spec") {
		t.Error("expected spec in output")
	}
}

func TestRenderATDDBuild(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `ATDD Build for: {{.Bead.Title}}
Acceptance tests have been written.
Make them pass.
Model: {{.Model}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_atdd_build.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &Context{
		Bead: &bead.Bead{
			ID:              "test-1",
			Title:           "Test feature",
			Description:     "Implement test feature",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
		Model:     "opus",
		Iteration: 1,
	}

	result, err := r.RenderATDDBuild(ctx)
	if err != nil {
		t.Fatalf("RenderATDDBuild() error = %v", err)
	}
	if result == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(result, "Test feature") {
		t.Error("expected bead title in output")
	}
	if !strings.Contains(result, "Acceptance tests have been written") {
		t.Error("expected ATDD instruction in output")
	}
	if !strings.Contains(result, "opus") {
		t.Error("expected model in output")
	}
}

func TestRenderRefactor(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Refactor code for: {{.Bead.Title}}
Review for code quality improvements.
Do not change behavior.
Iteration: {{.Iteration}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &Context{
		Bead: &bead.Bead{
			ID:              "test-1",
			Title:           "Refactor authentication",
			Description:     "Clean up auth code",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
		Iteration: 2,
	}

	result, err := r.RenderRefactor(ctx)
	if err != nil {
		t.Fatalf("RenderRefactor() error = %v", err)
	}
	if result == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(result, "Refactor authentication") {
		t.Error("expected bead title in output")
	}
	if !strings.Contains(result, "Do not change behavior") {
		t.Error("expected refactor instruction in output")
	}
	if !strings.Contains(result, "Iteration: 2") {
		t.Error("expected iteration in output")
	}
}

func TestRenderDecomposeAntiOverlapGuidance(t *testing.T) {
	// Use the real PROMPT_decompose.md template from the project
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")

	// Verify template exists
	templatePath := filepath.Join(templatesDir, "PROMPT_decompose.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skipf("skipping: real template not found at %s", templatePath)
	}

	r := &Renderer{templatesDir: templatesDir}

	testBead := &bead.Bead{
		ID:              "test-123",
		Title:           "Complex feature",
		Priority:        1,
		Description:     "Implement a complex feature",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	t.Run("anti-overlap guidance always present", func(t *testing.T) {
		ctx := &DecomposeContext{
			Bead:       testBead,
			ATDDActive: false,
		}
		result, err := r.RenderDecompose(ctx)
		if err != nil {
			t.Fatalf("RenderDecompose() error = %v", err)
		}
		if !strings.Contains(result, "Avoiding Sibling Overlap") {
			t.Error("expected anti-overlap guidance in output")
		}
		if !strings.Contains(result, "would this task's acceptance criteria still fail") {
			t.Error("expected cross-check question in output")
		}
	})

	t.Run("ATDD guidance absent when ATDDActive=false", func(t *testing.T) {
		ctx := &DecomposeContext{
			Bead:       testBead,
			ATDDActive: false,
		}
		result, err := r.RenderDecompose(ctx)
		if err != nil {
			t.Fatalf("RenderDecompose() error = %v", err)
		}
		if strings.Contains(result, "ATDD Active") {
			t.Error("ATDD guidance should NOT appear when ATDDActive=false")
		}
		if strings.Contains(result, "No Test-Only Beads") {
			t.Error("test-only bead suppression should NOT appear when ATDDActive=false")
		}
	})

	t.Run("ATDD guidance present when ATDDActive=true", func(t *testing.T) {
		ctx := &DecomposeContext{
			Bead:       testBead,
			ATDDActive: true,
		}
		result, err := r.RenderDecompose(ctx)
		if err != nil {
			t.Fatalf("RenderDecompose() error = %v", err)
		}
		if !strings.Contains(result, "ATDD Active") {
			t.Error("expected ATDD guidance when ATDDActive=true")
		}
		if !strings.Contains(result, "No Test-Only Beads") {
			t.Error("expected test-only bead suppression when ATDDActive=true")
		}
		if !strings.Contains(result, "Do NOT create sub-tasks whose sole purpose is writing tests") {
			t.Error("expected explicit test-only bead suppression instruction")
		}
	})

	t.Run("default DecomposeContext has no ATDD guidance", func(t *testing.T) {
		ctx := &DecomposeContext{
			Bead: testBead,
		}
		result, err := r.RenderDecompose(ctx)
		if err != nil {
			t.Fatalf("RenderDecompose() error = %v", err)
		}
		// Zero-value ATDDActive (false) should behave same as explicit false
		if strings.Contains(result, "ATDD Active") {
			t.Error("ATDD guidance should NOT appear with default (zero-value) ATDDActive")
		}
		// But anti-overlap should still be present
		if !strings.Contains(result, "Avoiding Sibling Overlap") {
			t.Error("expected anti-overlap guidance even with default context")
		}
	})
}

func TestRenderTDDBuild(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `TDD Build for: {{.Bead.Title}}
Follow red-green-refactor cycle.
Write ONE test at a time.
Model: {{.Model}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_build.md"), []byte(tmpl), 0644)

	r := &Renderer{templatesDir: templatesDir}

	ctx := &Context{
		Bead: &bead.Bead{
			ID:              "test-1",
			Title:           "Implement feature",
			Description:     "Build with TDD",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
		Model:     "sonnet",
		Iteration: 1,
	}

	result, err := r.RenderTDDBuild(ctx)
	if err != nil {
		t.Fatalf("RenderTDDBuild() error = %v", err)
	}
	if result == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(result, "Implement feature") {
		t.Error("expected bead title in output")
	}
	if !strings.Contains(result, "red-green-refactor") {
		t.Error("expected TDD instruction in output")
	}
	if !strings.Contains(result, "sonnet") {
		t.Error("expected model in output")
	}
}

func TestMethodologyPhaseShaping_TemplateCompatibility(t *testing.T) {
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	if _, err := os.Stat(filepath.Join(templatesDir, "PROMPT_acceptance_tests.md")); os.IsNotExist(err) {
		t.Skipf("skipping: real templates not found at %s", templatesDir)
	}

	r := &Renderer{templatesDir: templatesDir}
	base := testMethodologyContext()

	tests := []struct {
		name         string
		methodName   string
		renderName   string
		renderFn     func(*Context) (string, error)
		wantContains []string
	}{
		{
			name:       "red shaped context renders acceptance template",
			methodName: "ShapeRedPhaseContext",
			renderName: "RenderAcceptanceTests",
			renderFn:   r.RenderAcceptanceTests,
			wantContains: []string{
				"Rules (Non-Negotiable)",
				"Current Task",
				"Specification",
			},
		},
		{
			name:       "green shaped context renders atdd build template",
			methodName: "ShapeGreenPhaseContext",
			renderName: "RenderATDDBuild",
			renderFn:   r.RenderATDDBuild,
			wantContains: []string{
				"Rules (Non-Negotiable)",
				"Current Task",
				"Acceptance tests have been written",
			},
		},
		{
			name:       "refactor shaped context renders refactor template",
			methodName: "ShapeRefactorPhaseContext",
			renderName: "RenderRefactor",
			renderFn:   r.RenderRefactor,
			wantContains: []string{
				"Refactoring Phase",
				"Rules (Non-Negotiable)",
				"Do not change behavior",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := reflect.ValueOf(r).MethodByName(tt.methodName)
			if !method.IsValid() {
				t.Fatalf("Renderer.%s method not found for %s compatibility test", tt.methodName, tt.renderName)
			}

			if method.Type().NumIn() != 1 || method.Type().In(0) != reflect.TypeOf(&Context{}) {
				t.Fatalf("Renderer.%s should accept *Context", tt.methodName)
			}

			results := method.Call([]reflect.Value{reflect.ValueOf(base)})
			if len(results) == 0 {
				t.Fatalf("Renderer.%s returned no values", tt.methodName)
			}

			shaped, ok := results[0].Interface().(*Context)
			if !ok || shaped == nil {
				t.Fatalf("Renderer.%s first return value must be non-nil *Context", tt.methodName)
			}

			if len(results) == 2 && !results[1].IsNil() {
				err, _ := results[1].Interface().(error)
				t.Fatalf("Renderer.%s returned error: %v", tt.methodName, err)
			}

			rendered, err := tt.renderFn(shaped)
			if err != nil {
				t.Fatalf("%s() error with shaped context: %v", tt.renderName, err)
			}

			for _, needle := range tt.wantContains {
				if !strings.Contains(rendered, needle) {
					t.Errorf("%s output missing expected content %q after %s shaping", tt.renderName, needle, tt.methodName)
				}
			}
		})
	}
}
