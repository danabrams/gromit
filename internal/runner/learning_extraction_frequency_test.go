package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestExtractSuccessLearning_SkipsHaikuTierBeads verifies that learning extraction
// is skipped when the bead was processed with a haiku-tier model.
// Expected failure: extractSuccessLearning does not check bc.tier before invoking the provider
func TestExtractSuccessLearning_SkipsHaikuTierBeads(t *testing.T) {
	var buf strings.Builder
	providerInvoked := false

	mockProvider := &mockProviderForRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			providerInvoked = true
			return &provider.Result{
				Success: true,
				Output:  `{"learning": "Test learning", "category": "patterns"}`,
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	lf, _ := learnings.NewFile(t.TempDir())
	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "learning prompt", nil
		},
	}

	learnFromSuccessEnabled := true
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessEnabled,
			},
		},
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test",
			Description: "Test description",
		},
		tier: provider.TierLow, // haiku tier
	}

	r.extractSuccessLearning(context.Background(), bc)

	// Should NOT invoke provider for haiku-tier beads
	if providerInvoked {
		t.Error("expected learning extraction to be skipped for haiku-tier bead, but provider was invoked")
	}

	// Should NOT log success learning messages
	if strings.Contains(buf.String(), "Success learning extracted") {
		t.Error("expected no learning extraction log for haiku-tier bead")
	}
}

// TestExtractSuccessLearning_RunsForNonHaikuTierBeads verifies that learning extraction
// still runs for medium and high tier beads.
// Expected failure: extractSuccessLearning does not check bc.tier, so this passes regardless
func TestExtractSuccessLearning_RunsForNonHaikuTierBeads(t *testing.T) {
	tests := []struct {
		name string
		tier string
	}{
		{"sonnet tier", provider.TierMedium},
		{"opus tier", provider.TierHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			providerInvoked := false

			mockProvider := &mockProviderForRunner{
				FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
					providerInvoked = true
					return &provider.Result{
						Success: true,
						Output:  `{"learning": "Test learning", "category": "patterns"}`,
					}, nil
				},
			}

			mockRouter := provider.NewSingleProviderRouter(mockProvider)
			lf, _ := learnings.NewFile(t.TempDir())
			mockRend := &mockPromptRenderer{
				LearningsFile: lf,
				RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
					return "learning prompt", nil
				},
			}

			learnFromSuccessEnabled := true
			r := &Runner{
				cfg: &config.Config{
					Loop: config.LoopConfig{
						LearnFromSuccess: &learnFromSuccessEnabled,
					},
				},
				router:   mockRouter,
				renderer: mockRend,
				output:   &buf,
			}

			bc := &beadContext{
				bead: &bead.Bead{
					ID:          "test-1",
					Title:       "Test",
					Description: "Test description",
				},
				tier: tt.tier,
			}

			r.extractSuccessLearning(context.Background(), bc)

			// SHOULD invoke provider for non-haiku tiers
			if !providerInvoked {
				t.Errorf("expected learning extraction to run for %s, but provider was not invoked", tt.name)
			}
		})
	}
}

// TestExtractSuccessLearning_SkipsForKnownPackages verifies that learning extraction
// is skipped when the bead only touches packages that have been seen in the current run.
// Expected failure: Runner.touchedPackages and beadContext.touchedPackages fields do not exist yet
func TestExtractSuccessLearning_SkipsForKnownPackages(t *testing.T) {
	t.Skip("TODO: implement after touchedPackages field is added to Runner and beadContext")
	// This test will be enabled after implementation adds:
	// - Runner.touchedPackages map[string]bool field
	// - beadContext.touchedPackages []string field
	// - extractSuccessLearning logic to check r.touchedPackages
}

// TestExtractSuccessLearning_RunsForNewPackages verifies that learning extraction
// runs when the bead touches a package not previously seen in the current run.
// Expected failure: Runner.touchedPackages and beadContext.touchedPackages fields do not exist yet
func TestExtractSuccessLearning_RunsForNewPackages(t *testing.T) {
	t.Skip("TODO: implement after touchedPackages field is added to Runner and beadContext")
	// This test will be enabled after implementation adds:
	// - Runner.touchedPackages map[string]bool field
	// - beadContext.touchedPackages []string field
	// - extractSuccessLearning logic to check hasNewPackages(bc.touchedPackages)
}

// TestExtractSuccessLearning_AlwaysRunsOnFailure verifies that learning extraction
// runs for failed beads regardless of tier or touched packages.
// Expected failure: This behavior doesn't need changes; it already works via extractLearning()
func TestExtractSuccessLearning_AlwaysRunsOnFailure(t *testing.T) {
	// This test documents that failure learning extraction is NOT affected by the new rules.
	// extractLearning() is called during analyzeAndHandleFailure and should continue to run
	// regardless of tier or package novelty. This test exists to ensure we don't accidentally
	// break failure learning when implementing success learning filtering.

	// extractLearning already has comprehensive tests in process_test.go.
	// This test just documents that the filtering rules only apply to extractSuccessLearning,
	// not to extractLearning (which handles failure scenarios).
}

// TestBeadContext_TouchedPackages verifies that beadContext tracks touched packages.
// Expected failure: beadContext.touchedPackages field does not exist yet
func TestBeadContext_TouchedPackages(t *testing.T) {
	t.Skip("TODO: implement after touchedPackages field is added to beadContext")
	// This test will be enabled after implementation adds:
	// - beadContext.touchedPackages []string field
}

// TestRunner_TouchedPackages verifies that Runner tracks touched packages across iterations.
// Expected failure: Runner.touchedPackages field does not exist yet
func TestRunner_TouchedPackages(t *testing.T) {
	t.Skip("TODO: implement after touchedPackages field is added to Runner")
	// This test will be enabled after implementation adds:
	// - Runner.touchedPackages map[string]bool field
}

// TestDetectTouchedPackages verifies that detectTouchedPackages extracts Go package paths
// from git diff output.
func TestDetectTouchedPackages(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		expected []string
	}{
		{
			name:     "empty diff",
			diff:     "",
			expected: []string{},
		},
		{
			name: "single file",
			diff: `diff --git a/internal/runner/process.go b/internal/runner/process.go
index 123..456 789
--- a/internal/runner/process.go
+++ b/internal/runner/process.go
@@ -1,3 +1,4 @@`,
			expected: []string{"internal/runner"},
		},
		{
			name: "multiple files same package",
			diff: `diff --git a/internal/runner/process.go b/internal/runner/process.go
index 123..456 789
--- a/internal/runner/process.go
+++ b/internal/runner/process.go
@@ -1,3 +1,4 @@
diff --git a/internal/runner/validate.go b/internal/runner/validate.go
index 789..abc def`,
			expected: []string{"internal/runner"},
		},
		{
			name: "multiple files different packages",
			diff: `diff --git a/internal/runner/process.go b/internal/runner/process.go
index 123..456 789
--- a/internal/runner/process.go
+++ b/internal/runner/process.go
@@ -1,3 +1,4 @@
diff --git a/internal/config/config.go b/internal/config/config.go
index 789..abc def`,
			expected: []string{"internal/runner", "internal/config"},
		},
		{
			name: "ignores non-go files",
			diff: `diff --git a/README.md b/README.md
index 123..456 789
--- a/README.md
+++ b/README.md
@@ -1,3 +1,4 @@
diff --git a/internal/runner/process.go b/internal/runner/process.go
index 789..abc def`,
			expected: []string{"internal/runner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectTouchedPackages(tt.diff)
			if len(result) != len(tt.expected) {
				t.Errorf("detectTouchedPackages() returned %d packages, expected %d", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			for i, pkg := range tt.expected {
				if result[i] != pkg {
					t.Errorf("detectTouchedPackages()[%d] = %q, want %q", i, result[i], pkg)
				}
			}
		})
	}
}

// TestUpdateTouchedPackages verifies that updateTouchedPackages adds newly touched
// packages to the runner's tracking map.
// Expected failure: Runner.updateTouchedPackages method does not exist yet
func TestUpdateTouchedPackages(t *testing.T) {
	t.Skip("TODO: implement after updateTouchedPackages method is added")
	// This test will be enabled after implementation adds:
	// - Runner.updateTouchedPackages(packages []string) method
}

// TestHasNewPackages verifies that hasNewPackages returns true if any package
// in the list is not in the runner's touched packages map.
// Expected failure: Runner.hasNewPackages method does not exist yet
func TestHasNewPackages(t *testing.T) {
	t.Skip("TODO: implement after hasNewPackages method is added")
	// This test will be enabled after implementation adds:
	// - Runner.hasNewPackages(packages []string) bool method
}
