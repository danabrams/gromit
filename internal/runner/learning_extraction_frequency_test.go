package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// mockSuccessLearningResult adapts provider.Result to escalation.SuccessLearningResult.
type mockSuccessLearningResult struct {
	success bool
	output  string
}

func (r *mockSuccessLearningResult) IsSuccess() bool   { return r.success }
func (r *mockSuccessLearningResult) GetOutput() string { return r.output }

// mockSuccessLearningProvider satisfies escalation.SuccessLearningProvider.
type mockSuccessLearningProvider struct {
	RunFn func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error)
}

func (p *mockSuccessLearningProvider) Run(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
	if p.RunFn != nil {
		return p.RunFn(ctx, prompt, tier)
	}
	return &mockSuccessLearningResult{success: true, output: `{}`}, nil
}

// mockSuccessLearningRouter satisfies escalation.SuccessLearningRouter.
type mockSuccessLearningRouter struct {
	provider *mockSuccessLearningProvider
}

func (r *mockSuccessLearningRouter) Select(phase, tier string) (escalation.SuccessLearningProvider, string) {
	return r.provider, tier
}

// TestExtractSuccessLearning_SkipsHaikuTierBeads verifies that learning extraction
// is skipped when the bead was processed with a haiku-tier model.
// Expected failure: extractSuccessLearning does not check bc.Tier before invoking the provider
func TestExtractSuccessLearning_SkipsHaikuTierBeads(t *testing.T) {
	providerInvoked := false

	mp := &mockSuccessLearningProvider{
		RunFn: func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
			providerInvoked = true
			return &mockSuccessLearningResult{
				success: true,
				output:  `{"learning": "Test learning", "category": "patterns"}`,
			}, nil
		},
	}

	router := &mockSuccessLearningRouter{provider: mp}
	lf, _ := learnings.NewFile(t.TempDir())

	learnFromSuccessEnabled := true
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test",
			Description: "Test description",
		},
		Tier: provider.TierLow, // haiku tier
	}

	escalation.ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil, nil)

	// Should NOT invoke provider for haiku-tier beads
	if providerInvoked {
		t.Error("expected learning extraction to be skipped for haiku-tier bead, but provider was invoked")
	}
}

// TestExtractSuccessLearning_RunsForNonHaikuTierBeads verifies that learning extraction
// still runs for medium and high tier beads.
// Expected failure: extractSuccessLearning does not check bc.Tier, so this passes regardless
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
			providerInvoked := false

			mp := &mockSuccessLearningProvider{
				RunFn: func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
					providerInvoked = true
					return &mockSuccessLearningResult{
						success: true,
						output:  `{"learning": "Test learning", "category": "patterns"}`,
					}, nil
				},
			}

			router := &mockSuccessLearningRouter{provider: mp}
			lf, _ := learnings.NewFile(t.TempDir())

			learnFromSuccessEnabled := true
			cfg := &config.Config{
				Loop: config.LoopConfig{
					LearnFromSuccess: &learnFromSuccessEnabled,
				},
			}

			bc := &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:          "test-1",
					Title:       "Test",
					Description: "Test description",
				},
				Tier: tt.tier,
			}

			escalation.ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil, nil)

			// SHOULD invoke provider for non-haiku tiers
			if !providerInvoked {
				t.Errorf("expected learning extraction to run for %s, but provider was not invoked", tt.name)
			}
		})
	}
}

// TestExtractSuccessLearning_SkipsForKnownPackages verifies that learning extraction
// is skipped when the bead only touches packages that have been seen in the current run.
func TestExtractSuccessLearning_SkipsForKnownPackages(t *testing.T) {
	providerInvoked := false

	mp := &mockSuccessLearningProvider{
		RunFn: func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
			providerInvoked = true
			return &mockSuccessLearningResult{
				success: true,
				output:  `{"learning": "Test learning", "category": "patterns"}`,
			}, nil
		},
	}

	router := &mockSuccessLearningRouter{provider: mp}
	lf, _ := learnings.NewFile(t.TempDir())

	learnFromSuccessEnabled := true
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test",
			Description: "Test description",
		},
		Tier:            provider.TierMedium,         // non-haiku tier
		TouchedPackages: []string{"internal/runner"}, // all packages already seen
	}

	seenPackages := map[string]bool{
		"internal/runner": true,
		"internal/config": true,
	}
	escalation.ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil, seenPackages)

	// Should NOT invoke provider when all packages are already seen
	if providerInvoked {
		t.Error("expected learning extraction to be skipped for known packages, but provider was invoked")
	}
}

// TestExtractSuccessLearning_RunsForNewPackages verifies that learning extraction
// runs when the bead touches a package not previously seen in the current run.
func TestExtractSuccessLearning_RunsForNewPackages(t *testing.T) {
	providerInvoked := false

	mp := &mockSuccessLearningProvider{
		RunFn: func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
			providerInvoked = true
			return &mockSuccessLearningResult{
				success: true,
				output:  `{"learning": "Test learning", "category": "patterns"}`,
			}, nil
		},
	}

	router := &mockSuccessLearningRouter{provider: mp}
	lf, _ := learnings.NewFile(t.TempDir())

	learnFromSuccessEnabled := true
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test",
			Description: "Test description",
		},
		Tier:            provider.TierMedium,                            // non-haiku tier
		TouchedPackages: []string{"internal/runner", "internal/config"}, // internal/config is new
	}

	seenPackages := map[string]bool{
		"internal/runner": true, // only runner is seen; config is new
	}
	escalation.ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil, seenPackages)

	// SHOULD invoke provider when at least one package is new
	if !providerInvoked {
		t.Error("expected learning extraction to run for new packages, but provider was not invoked")
	}
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

// TestBeadContext_TouchedPackages verifies that BeadContext tracks touched packages.
func TestBeadContext_TouchedPackages(t *testing.T) {
	bc := &runtypes.BeadContext{
		TouchedPackages: []string{"internal/runner", "internal/config"},
	}

	if len(bc.TouchedPackages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(bc.TouchedPackages))
	}
	if bc.TouchedPackages[0] != "internal/runner" {
		t.Errorf("expected first package to be 'internal/runner', got %q", bc.TouchedPackages[0])
	}
	if bc.TouchedPackages[1] != "internal/config" {
		t.Errorf("expected second package to be 'internal/config', got %q", bc.TouchedPackages[1])
	}
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
		{
			name: "root-level go file returns dot",
			diff: `diff --git a/main.go b/main.go
index 123..456 789
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@`,
			expected: []string{"."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := methodology.DetectTouchedPackages(tt.diff)
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
func TestUpdateTouchedPackages(t *testing.T) {
	r := &Runner{
		touchedPackages: map[string]bool{
			"internal/runner": true,
		},
	}

	r.updateTouchedPackages([]string{"internal/config", "internal/bead"})

	if !r.touchedPackages["internal/config"] {
		t.Error("expected internal/config to be added to touchedPackages")
	}
	if !r.touchedPackages["internal/bead"] {
		t.Error("expected internal/bead to be added to touchedPackages")
	}
	if !r.touchedPackages["internal/runner"] {
		t.Error("expected internal/runner to remain in touchedPackages")
	}
}

// TestHasNewPackages verifies that hasNewPackages returns true if any package
// in the list is not in the runner's touched packages map.
func TestHasNewPackages(t *testing.T) {
	tests := []struct {
		name            string
		touchedPackages map[string]bool
		packages        []string
		expected        bool
	}{
		{
			name:            "empty runner map, non-empty packages",
			touchedPackages: map[string]bool{},
			packages:        []string{"internal/runner"},
			expected:        true,
		},
		{
			name:            "all packages already seen",
			touchedPackages: map[string]bool{"internal/runner": true, "internal/config": true},
			packages:        []string{"internal/runner", "internal/config"},
			expected:        false,
		},
		{
			name:            "some packages new",
			touchedPackages: map[string]bool{"internal/runner": true},
			packages:        []string{"internal/runner", "internal/config"},
			expected:        true,
		},
		{
			name:            "empty packages list",
			touchedPackages: map[string]bool{"internal/runner": true},
			packages:        []string{},
			expected:        false,
		},
		{
			name:            "nil runner map",
			touchedPackages: nil,
			packages:        []string{"internal/runner"},
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				touchedPackages: tt.touchedPackages,
			}
			result := r.hasNewPackages(tt.packages)
			if result != tt.expected {
				t.Errorf("hasNewPackages() = %v, want %v", result, tt.expected)
			}
		})
	}
}
