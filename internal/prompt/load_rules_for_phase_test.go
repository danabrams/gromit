package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rulesWithAnnotations returns a RULES.md with phase annotations on ## headers,
// matching the expected annotated format described in the task spec.
func rulesWithAnnotations() string {
	return `# Rules

These are non-negotiable constraints for this project.

## Code Style <!-- phases: red, build, green, refactor, review -->

- Use ` + "`go fmt`" + ` standard formatting
- Use error return values, not panics

## Safety <!-- phases: red, build, green, refactor, review -->

- Never commit secrets
- Never delete data without confirmation

## Test Quality <!-- phases: red, build, green, refactor, review -->

- Acceptance tests must test behavior through the public API
- Do not test Go standard library behavior

## Build Process <!-- phases: build -->

- Always run tests before committing
- Follow existing patterns in the codebase
- Beads that touch 6+ files should be split
`
}

// setupRendererWithRules creates a Renderer with a RULES.md file at the given path.
func setupRendererWithRules(t *testing.T, rulesContent string) *Renderer {
	t.Helper()
	tmpDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("writing test RULES.md: %v", err)
	}
	return &Renderer{
		rulesPath: rulesPath,
		gromitDir: tmpDir,
	}
}

func TestLoadRulesForPhase(t *testing.T) {
	r := setupRendererWithRules(t, rulesWithAnnotations())

	tests := []struct {
		name            string
		phase           string
		wantSections    []string
		notWantSections []string
	}{
		{
			name:  "build phase returns all sections",
			phase: "build",
			wantSections: []string{
				"## Code Style",
				"## Safety",
				"## Test Quality",
				"## Build Process",
			},
			notWantSections: nil,
		},
		{
			name:  "review phase excludes build process section",
			phase: "review",
			wantSections: []string{
				"## Code Style",
				"## Safety",
				"## Test Quality",
			},
			notWantSections: []string{
				"## Build Process",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.LoadRulesForPhase(tt.phase)
			if err != nil {
				t.Fatalf("LoadRulesForPhase(%q) error = %v", tt.phase, err)
			}

			for _, section := range tt.wantSections {
				if !strings.Contains(result, section) {
					t.Errorf("LoadRulesForPhase(%q) missing expected section %q", tt.phase, section)
				}
			}
			for _, section := range tt.notWantSections {
				if strings.Contains(result, section) {
					t.Errorf("LoadRulesForPhase(%q) should not contain section %q", tt.phase, section)
				}
			}
		})
	}
}

func TestLoadRulesForPhaseReviewExcludesProcessContent(t *testing.T) {
	r := setupRendererWithRules(t, rulesWithAnnotations())

	result, err := r.LoadRulesForPhase("review")
	if err != nil {
		t.Fatalf("LoadRulesForPhase(review) error = %v", err)
	}

	// Verify process-specific content is excluded
	processOnlyContent := []string{
		"Always run tests before committing",
		"Follow existing patterns in the codebase",
		"Beads that touch 6+ files should be split",
	}
	for _, content := range processOnlyContent {
		if strings.Contains(result, content) {
			t.Errorf("LoadRulesForPhase(review) should not contain process content %q", content)
		}
	}

	// Verify non-process content IS included
	reviewContent := []string{
		"go fmt",
		"Never commit secrets",
		"Acceptance tests must test behavior through the public API",
	}
	for _, content := range reviewContent {
		if !strings.Contains(result, content) {
			t.Errorf("LoadRulesForPhase(review) missing expected content %q", content)
		}
	}
}

func TestLoadRulesForPhaseBuildReturnsSameAsFullRules(t *testing.T) {
	// Since all sections are tagged with "build", LoadRulesForPhase("build")
	// should return content containing every section from the rules file.
	r := setupRendererWithRules(t, rulesWithAnnotations())

	buildResult, err := r.LoadRulesForPhase("build")
	if err != nil {
		t.Fatalf("LoadRulesForPhase(build) error = %v", err)
	}

	// All four section headers must be present
	expectedHeaders := []string{"## Code Style", "## Safety", "## Test Quality", "## Build Process"}
	for _, header := range expectedHeaders {
		if !strings.Contains(buildResult, header) {
			t.Errorf("LoadRulesForPhase(build) missing section header %q", header)
		}
	}
}

func TestLoadRulesForPhaseReviewIsSmallerThanBuild(t *testing.T) {
	r := setupRendererWithRules(t, rulesWithAnnotations())

	buildResult, err := r.LoadRulesForPhase("build")
	if err != nil {
		t.Fatalf("LoadRulesForPhase(build) error = %v", err)
	}

	reviewResult, err := r.LoadRulesForPhase("review")
	if err != nil {
		t.Fatalf("LoadRulesForPhase(review) error = %v", err)
	}

	if len(reviewResult) >= len(buildResult) {
		t.Errorf("review rules (%d bytes) should be smaller than build rules (%d bytes)",
			len(reviewResult), len(buildResult))
	}
}

func TestLoadRulesForPhaseRedAndRefactorExcludeBuildProcess(t *testing.T) {
	r := setupRendererWithRules(t, rulesWithAnnotations())

	for _, phase := range []string{"red", "refactor"} {
		phase := phase
		t.Run(phase, func(t *testing.T) {
			result, err := r.LoadRulesForPhase(phase)
			if err != nil {
				t.Fatalf("LoadRulesForPhase(%q) error = %v", phase, err)
			}
			if !strings.Contains(result, "## Code Style") {
				t.Fatalf("LoadRulesForPhase(%q) missing shared section %q", phase, "## Code Style")
			}
			if strings.Contains(result, "## Build Process") {
				t.Fatalf("LoadRulesForPhase(%q) should exclude build-only section %q", phase, "## Build Process")
			}
		})
	}
}

func TestLoadRulesForPhaseUnannotatedSectionsIncludedInAllPhases(t *testing.T) {
	// Sections without phase annotations should be included for all phases.
	rules := `# Rules

These are non-negotiable constraints.

## Code Style <!-- phases: build, review -->

- Use go fmt standard formatting

## Unannotated Section

- This has no phase annotation and should appear in all phases

## Process <!-- phases: build -->

- Always run tests before committing
`
	r := setupRendererWithRules(t, rules)

	tests := []struct {
		name  string
		phase string
	}{
		{"build includes unannotated", "build"},
		{"review includes unannotated", "review"},
		{"validate includes unannotated", "validate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.LoadRulesForPhase(tt.phase)
			if err != nil {
				t.Fatalf("LoadRulesForPhase(%q) error = %v", tt.phase, err)
			}
			if !strings.Contains(result, "## Unannotated Section") {
				t.Errorf("LoadRulesForPhase(%q) should include unannotated sections", tt.phase)
			}
			if !strings.Contains(result, "This has no phase annotation") {
				t.Errorf("LoadRulesForPhase(%q) should include unannotated section content", tt.phase)
			}
		})
	}
}

func TestLoadRulesForPhaseNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadRulesForPhase("build")
	if err == nil {
		t.Error("expected error for nil renderer, got nil")
	}
}

func TestLoadRulesForPhaseMissingFile(t *testing.T) {
	r := &Renderer{
		rulesPath: filepath.Join(t.TempDir(), "nonexistent", "RULES.md"),
	}

	result, err := r.LoadRulesForPhase("build")
	if err != nil {
		t.Fatalf("LoadRulesForPhase with missing file should not error, got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for missing RULES.md, got %q", result)
	}
}

func TestLoadRulesForPhasePreservesAnnotationFreeOutput(t *testing.T) {
	// The returned content should have the phase annotation comments stripped
	// from the headers so that Claude doesn't see the meta-annotations.
	r := setupRendererWithRules(t, rulesWithAnnotations())

	result, err := r.LoadRulesForPhase("build")
	if err != nil {
		t.Fatalf("LoadRulesForPhase(build) error = %v", err)
	}

	if strings.Contains(result, "<!-- phases:") {
		t.Error("LoadRulesForPhase output should not contain phase annotation comments")
	}
}
