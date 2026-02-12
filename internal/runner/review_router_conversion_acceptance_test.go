//go:build acceptance

package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/state"
)

// TestAcceptance_RunLightReviewUsesRouterSelect verifies that runLightReview
// calls router.Select() with phase="review" and the tier from selectReviewTier().
func TestAcceptance_RunLightReviewUsesRouterSelect(t *testing.T) {
	_, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()

	// Track provider.Run() calls to verify router was used
	var capturedTier string
	providerCalled := false

	mockProvider := &mockProviderWithRouterTracking{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerCalled = true
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-model",
				Output:  `{"passed": true, "summary": "LGTM: The implementation looks good.", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	b := &bead.Bead{
		ID:       "review-001",
		Title:    "Test bead for review",
		Priority: 1,
		Labels:   []string{},
	}

	// Provide mock claude client for backward compatibility
	mockClaude := &mockClaudeClientForReview{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		renderer: &mockPromptRenderer{
			RenderReviewFn: func(*prompt.ReviewContext) (string, error) {
				return "review this code", nil
			},
			LoadClaudeMDFn: func() (string, error) { return "CLAUDE.md", nil },
			LoadRulesFn:    func() (string, error) { return "RULES.md", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff content", nil },
	}

	_, err := r.runLightReview(context.Background(), b, nil, startCommit, "sonnet", 1, time.Time{})
	if err != nil {
		t.Fatalf("runLightReview() error = %v", err)
	}

	// Verify router was used (provider was called)
	if !providerCalled {
		t.Error("router.Select() was not called - runLightReview does not use router")
	}

	// Verify tier is selected by selectReviewTier (which defaults to selectTier for non-opus builds)
	expectedTier := r.selectTier(b)
	if capturedTier != expectedTier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, expectedTier)
	}
}

// TestAcceptance_RunLightReviewCallsProviderRun verifies that runLightReview
// invokes provider.Run() with the selected tier.
// Expected failure: provider.Run() method is not yet called by runLightReview - it uses r.claude.Run()
func TestAcceptance_RunLightReviewCallsProviderRun(t *testing.T) {
	_, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()

	providerRunCalled := false
	var capturedPrompt, capturedTier string

	mockProvider := &mockProviderWithRouterTracking{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerRunCalled = true
			capturedPrompt = prompt
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-model",
				Output:  `{"passed": true, "summary": "LGTM: OK", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	b := &bead.Bead{
		ID:       "review-002",
		Priority: 1,
	}

	expectedPrompt := "review this implementation"

	mockClaude := &mockClaudeClientForReview{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		renderer: &mockPromptRenderer{
			RenderReviewFn: func(*prompt.ReviewContext) (string, error) {
				return expectedPrompt, nil
			},
			LoadClaudeMDFn: func() (string, error) { return "", nil },
			LoadRulesFn:    func() (string, error) { return "", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff", nil },
	}

	_, err := r.runLightReview(context.Background(), b, nil, startCommit, "haiku", 1, time.Time{})
	if err != nil {
		t.Fatalf("runLightReview() error = %v", err)
	}

	if !providerRunCalled {
		t.Error("provider.Run() was not called - runLightReview does not invoke the provider")
	}

	if capturedPrompt != expectedPrompt {
		t.Errorf("provider.Run() prompt = %q, want %q", capturedPrompt, expectedPrompt)
	}

	expectedTier := r.selectTier(b)
	if capturedTier != expectedTier {
		t.Errorf("provider.Run() tier = %q, want %q", capturedTier, expectedTier)
	}
}

// TestAcceptance_RunLightReviewUsesHighTierForOpusBuild verifies that when the
// buildModel is "opus" and ShouldMatchBuildModel is true, runLightReview uses
// tier "high" instead of the default tier.
// Expected failure: runLightReview does not implement the tier selection logic for matching opus builds
func TestAcceptance_RunLightReviewUsesHighTierForOpusBuild(t *testing.T) {
	_, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	trueVal := true
	cfg.Review.MatchBuildModel = &trueVal // Enable matching build model

	var capturedTier string

	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-opus",
				Output:  `{"passed": true, "summary": "LGTM: OK", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	b := &bead.Bead{
		ID:       "review-003",
		Priority: 2, // Would normally use low tier
	}

	mockClaude := &mockClaudeClientForReview{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		renderer: &mockPromptRenderer{
			RenderReviewFn: func(*prompt.ReviewContext) (string, error) {
				return "review", nil
			},
			LoadClaudeMDFn: func() (string, error) { return "", nil },
			LoadRulesFn:    func() (string, error) { return "", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff", nil },
	}

	// Call with buildModel="opus"
	_, err := r.runLightReview(context.Background(), b, nil, startCommit, "opus", 1, time.Time{})
	if err != nil {
		t.Fatalf("runLightReview() error = %v", err)
	}

	// Verify tier is "high" when build model is opus and matching is enabled
	if capturedTier != provider.TierHigh {
		t.Errorf("router.Select() tier = %q, want %q when buildModel=opus", capturedTier, provider.TierHigh)
	}
}

// TestAcceptance_RunThoroughReviewUsesRouterSelect verifies that runThoroughReview
// calls router.Select() with phase="review" and tier="high".
// Expected failure: runThoroughReview does not yet call router.Select() - it still uses r.claude.Run()
func TestAcceptance_RunThoroughReviewUsesRouterSelect(t *testing.T) {
	tmpDir, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = false // Disable validation for simpler test

	var capturedTier string

	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-opus",
				Output:  `{"passed": true, "summary": "LGTM: Thorough review complete", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	// Create state file with last review commit
	stateDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(stateDir, 0755)
	sf, _ := state.NewFile(stateDir)
	sf.RecordReview(startCommit, 0)

	mockClaude := &mockClaudeClientForReview{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		renderer: &mockPromptRenderer{
			RenderThoroughReviewFn: func(*prompt.ThoroughReviewContext) (string, error) {
				return "thorough review", nil
			},
			LoadClaudeMDFn: func() (string, error) { return "", nil },
			LoadRulesFn:    func() (string, error) { return "", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff content", nil },
	}

	r.runThoroughReview(context.Background(), sf, 1, time.Time{})

	// Verify router was used (provider was called)
	if capturedTier == "" {
		t.Error("router.Select() was not called - runThoroughReview does not use router")
	}

	// Verify tier is "high" for thorough review
	if capturedTier != provider.TierHigh {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, provider.TierHigh)
	}
}

// TestAcceptance_RunThoroughReviewCallsProviderRun verifies that runThoroughReview
// invokes provider.Run() with tier="high".
// Expected failure: provider.Run() method is not yet called by runThoroughReview - it uses r.claude.Run()
func TestAcceptance_RunThoroughReviewCallsProviderRun(t *testing.T) {
	tmpDir, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = false

	providerRunCalled := false
	var capturedTier string

	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerRunCalled = true
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-opus",
				Output:  `{"passed": true, "summary": "LGTM: OK", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	stateDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(stateDir, 0755)
	sf, _ := state.NewFile(stateDir)
	sf.RecordReview(startCommit, 0)

	mockClaude := &mockClaudeClientForReview{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		renderer: &mockPromptRenderer{
			RenderThoroughReviewFn: func(*prompt.ThoroughReviewContext) (string, error) {
				return "thorough review prompt", nil
			},
			LoadClaudeMDFn: func() (string, error) { return "", nil },
			LoadRulesFn:    func() (string, error) { return "", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff", nil },
	}

	r.runThoroughReview(context.Background(), sf, 1, time.Time{})

	if !providerRunCalled {
		t.Error("provider.Run() was not called - runThoroughReview does not invoke the provider")
	}

	if capturedTier != provider.TierHigh {
		t.Errorf("provider.Run() tier = %q, want %q", capturedTier, provider.TierHigh)
	}
}

// TestAcceptance_RunThoroughReviewValidationUsesRouterWithLowTier verifies that
// the validation sub-call in runThoroughReview uses router.Select() with
// phase="validate" and tier="low".
// Expected failure: runThoroughReview validation does not yet use router - it uses r.claude.RunValidation()
func TestAcceptance_RunThoroughReviewValidationUsesRouterWithLowTier(t *testing.T) {
	tmpDir, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"echo test"}

	var validationPhase, validationTier string
	validationCalled := false

	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			// Main review call - returns JSON with fixes_applied
			return &provider.Result{
				Success: true,
				Model:   "test-opus",
				Output:  `{"passed": true, "summary": "LGTM: Fixed issue", "fixes_applied": ["fixed test.go"], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	// Override RunValidation to track calls
	mockProviderWithValidation := &mockProviderWithValidationTracking{
		mockProviderWithRouterTracking: mockProvider,
		onValidationSelect: func(phase, tier string) {
			validationPhase = phase
			validationTier = tier
			validationCalled = true
		},
		runValidationResult: &provider.Result{
			Success: true,
			Model:   "test-haiku",
			Output:  "VALIDATION_PASSED",
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProviderWithValidation)

	stateDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(stateDir, 0755)
	sf, _ := state.NewFile(stateDir)
	sf.RecordReview(startCommit, 0)

	mockClaude := &mockClaudeClientForReview{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		claude: mockClaude,
		renderer: &mockPromptRenderer{
			RenderThoroughReviewFn: func(*prompt.ThoroughReviewContext) (string, error) {
				return "thorough review", nil
			},
			LoadClaudeMDFn: func() (string, error) { return "", nil },
			LoadRulesFn:    func() (string, error) { return "", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff", nil },
	}

	r.runThoroughReview(context.Background(), sf, 1, time.Time{})

	if !validationCalled {
		t.Error("router.Select() was not called for validation - runThoroughReview validation does not use router")
	}

	if validationPhase != "validate" {
		t.Errorf("validation router.Select() phase = %q, want %q", validationPhase, "validate")
	}

	if validationTier != provider.TierLow {
		t.Errorf("validation router.Select() tier = %q, want %q", validationTier, provider.TierLow)
	}
}

// mockProviderWithValidationTracking extends mockProviderWithRouterTracking
// to track validation calls separately
type mockProviderWithValidationTracking struct {
	*mockProviderWithRouterTracking
	onValidationSelect  func(phase, tier string)
	runValidationResult *provider.Result
	runValidationErr    error
}

func (m *mockProviderWithValidationTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.onValidationSelect != nil {
		// We can infer the phase is "validate" since this is RunValidation
		m.onValidationSelect("validate", tier)
	}
	if m.runValidationResult != nil {
		return m.runValidationResult, m.runValidationErr
	}
	return &provider.Result{Success: true, Model: "test-haiku", Output: "VALIDATION_PASSED"}, nil
}

// mockClaudeClientForReview provides backward-compatible claude client for review tests.
// Returns valid review JSON output that can be parsed.
type mockClaudeClientForReview struct{}

func (m *mockClaudeClientForReview) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	reviewJSON := `{"passed": true, "summary": "LGTM", "fixes_applied": [], "proposals": []}`
	return &claude.Result{Success: true, Model: model, Output: reviewJSON}, nil
}

func (m *mockClaudeClientForReview) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	reviewJSON := `{"passed": true, "summary": "LGTM", "fixes_applied": [], "proposals": []}`
	return &claude.Result{Success: true, Model: model, Output: reviewJSON}, nil
}

func (m *mockClaudeClientForReview) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	return &claude.Result{Success: true, Model: model, Output: "VALIDATION_PASSED"}, nil
}
