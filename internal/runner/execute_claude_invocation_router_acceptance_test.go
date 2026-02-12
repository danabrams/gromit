//go:build acceptance

package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/state"
)

// TestAcceptance_ExecuteClaudeInvocationUsesRouterSelect verifies that
// executeClaudeInvocation calls router.Select() with phase="build" and tier from selectTier().
// Expected failure: executeClaudeInvocation does not yet call router.Select() - it still uses r.claude.StreamRun()
func TestAcceptance_ExecuteClaudeInvocationUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()

	// Track router.Select() calls
	var capturedPhase, capturedTier string

	mockProvider := &mockProviderWithSelectTracking{
		onSelect: func(phase, tier string) {
			capturedPhase = phase
			capturedTier = tier
		},
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-001",
			Priority: 1,
			Labels:   []string{},
		},
		model:       "sonnet", // Legacy model name from setupBeadContext
		result:      &IterationResult{},
		buildPrompt: "implement feature X",
	}

	// Provide a mock claude client for backward compatibility (current implementation uses r.claude)
	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		output: io.Discard,
	}

	// Call executeClaudeInvocation
	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v", err)
	}

	// Verify router.Select() was called with correct phase
	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier matches selectTier(bead) result
	expectedTier := r.selectTier(bc.bead)
	if capturedTier != expectedTier {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, expectedTier)
	}
}

// makeTestRunnerConfig creates a minimal config for runner tests
func makeTestRunnerConfig() *config.Config {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// TestAcceptance_ExecuteClaudeInvocationCallsProviderStreamRun verifies that
// executeClaudeInvocation invokes provider.StreamRun() with the selected tier.
// Expected failure: provider.StreamRun() method does not exist yet - executeClaudeInvocation uses claude.StreamRun()
func TestAcceptance_ExecuteClaudeInvocationCallsProviderStreamRun(t *testing.T) {
	cfg := makeTestRunnerConfig()

	streamRunCalled := false
	var capturedPrompt, capturedTier string

	mockProvider := &mockProviderWithSelectTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCalled = true
			capturedPrompt = prompt
			capturedTier = tier
			return &provider.Result{Success: true, Model: "test-model", Output: "done"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-002",
			Priority: 1,
		},
		result:      &IterationResult{},
		buildPrompt: "implement authentication",
	}

	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		output: io.Discard,
	}

	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v", err)
	}

	if !streamRunCalled {
		t.Error("provider.StreamRun() was not called - executeClaudeInvocation does not invoke the provider")
	}

	if capturedPrompt != bc.buildPrompt {
		t.Errorf("provider.StreamRun() prompt = %q, want %q", capturedPrompt, bc.buildPrompt)
	}

	expectedTier := r.selectTier(bc.bead)
	if capturedTier != expectedTier {
		t.Errorf("provider.StreamRun() tier = %q, want %q", capturedTier, expectedTier)
	}
}

// TestAcceptance_ExecuteClaudeInvocationDetectsUsageLimitError verifies usage limit fallback
func TestAcceptance_ExecuteClaudeInvocationDetectsUsageLimitError(t *testing.T) {
	limitErr := errors.New("usage limit")
	callCount := 0
	var marked string

	p1 := &mockProviderWithUsageLimitTracking{
		name: "claude",
		streamRunFn: func(context.Context, string, string, io.Writer, provider.EventHandler, provider.ToolCallHandler) (*provider.Result, error) {
			callCount++
			if callCount == 1 {
				return &provider.Result{Success: false}, limitErr
			}
			return &provider.Result{Success: true, Model: "fallback"}, nil
		},
		isUsageLimitErrorFn: func(*provider.Result, error) bool { return callCount == 1 },
		onMarkUnavailable:   func(n string) { marked = n },
	}
	p2 := &mockProviderWithUsageLimitTracking{
		name: "openai",
		streamRunFn: func(context.Context, string, string, io.Writer, provider.EventHandler, provider.ToolCallHandler) (*provider.Result, error) {
			callCount++
			return &provider.Result{Success: true, Model: "fallback"}, nil
		},
	}

	r := &Runner{
		cfg: makeTestRunnerConfig(),
		router: provider.NewRouter(
			map[string]provider.Provider{"claude": p1, "openai": p2},
			map[string]string{"build": "any"},
			map[string]int{"claude": 50, "openai": 50},
			0,
			&mockStateForRouterTest{onSetUnavailable: func(n string, _ time.Time) { marked = n }},
		),
		output: io.Discard,
	}

	result, _, _, err := r.executeClaudeInvocation(context.Background(), &beadContext{
		bead:        &bead.Bead{ID: "t3", Priority: 1},
		result:      &IterationResult{},
		buildPrompt: "test",
	})

	if err != nil || callCount != 2 || marked != "claude" || !result.Success {
		t.Errorf("got err=%v calls=%d marked=%q success=%v", err, callCount, marked, result.Success)
	}
}

// TestAcceptance_ExecuteClaudeInvocationReturnsErrorWhenAllProvidersUnavailable verifies
// error when no providers available
func TestAcceptance_ExecuteClaudeInvocationReturnsErrorWhenAllProvidersUnavailable(t *testing.T) {
	r := &Runner{
		cfg:    makeTestRunnerConfig(),
		router: provider.NewRouter(map[string]provider.Provider{}, map[string]string{"build": "any"}, map[string]int{}, 0, &mockStateForRouterTest{}),
		output: io.Discard,
	}

	_, _, _, err := r.executeClaudeInvocation(context.Background(), &beadContext{
		bead:        &bead.Bead{ID: "t4", Priority: 1},
		result:      &IterationResult{},
		buildPrompt: "fail",
	})

	if err == nil || err.Error() == "" {
		t.Error("expected non-empty error when no providers available")
	}
}

// TestAcceptance_ExecuteClaudeInvocationUpdatesBeadContextModel verifies that
// the model name returned from router.Select() updates bc.model and bc.result.Model.
// Expected failure: executeClaudeInvocation does not update bc.model with the router-selected model
func TestAcceptance_ExecuteClaudeInvocationUpdatesBeadContextModel(t *testing.T) {
	cfg := makeTestRunnerConfig()

	routerReturnedModel := "provider-specific-model-name"

	mockProvider := &mockProviderWithSelectTracking{
		streamRunResult: &provider.Result{Success: true, Model: routerReturnedModel, Output: "done"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-005",
			Priority: 1,
		},
		model:       "legacy-model",
		result:      &IterationResult{Model: "legacy-model"},
		buildPrompt: "test model update",
	}

	mockClaude := &mockClaudeClientForProcess{}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
		output: io.Discard,
	}

	_, _, _, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation() error = %v", err)
	}

	// Verify bc.model was updated to router-selected model
	if bc.model != routerReturnedModel {
		t.Errorf("bc.model = %q, want %q (router-selected model)", bc.model, routerReturnedModel)
	}

	// Verify bc.result.Model was updated
	if bc.result.Model != routerReturnedModel {
		t.Errorf("bc.result.Model = %q, want %q (router-selected model)", bc.result.Model, routerReturnedModel)
	}
}

// mockProviderWithSelectTracking is a test double that tracks router.Select() calls
type mockProviderWithSelectTracking struct {
	name                string
	onSelect            func(phase, tier string)
	runFn               func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	streamRunFn         func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	streamRunResult     *provider.Result
	streamRunErr        error
	runValidationResult *provider.Result
}

func (m *mockProviderWithSelectTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderWithSelectTracking) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "mock-opus"
	case provider.TierMedium:
		return "mock-sonnet"
	case provider.TierLow:
		return "mock-haiku"
	default:
		return "mock-model"
	}
}

func (m *mockProviderWithSelectTracking) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if m.onSelect != nil {
		m.onSelect("", tier)
	}
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	modelName := "mock-model"
	if m.streamRunResult != nil && m.streamRunResult.Model != "" {
		modelName = m.streamRunResult.Model
	}
	return &provider.Result{Success: true, Model: modelName}, nil
}

func (m *mockProviderWithSelectTracking) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.onSelect != nil {
		m.onSelect("build", tier)
	}
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	if m.streamRunResult != nil {
		return m.streamRunResult, m.streamRunErr
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithSelectTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.runValidationResult != nil {
		return m.runValidationResult, nil
	}
	return &provider.Result{Success: true, Model: "mock-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderWithSelectTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

// mockProviderWithUsageLimitTracking is a test double for usage limit testing
type mockProviderWithUsageLimitTracking struct {
	name                string
	runFn               func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	streamRunFn         func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	isUsageLimitErrorFn func(result *provider.Result, err error) bool
	onMarkUnavailable   func(name string)
	runValidationResult *provider.Result
}

func (m *mockProviderWithUsageLimitTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProviderWithUsageLimitTracking) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return m.name + "-high"
	case provider.TierMedium:
		return m.name + "-medium"
	case provider.TierLow:
		return m.name + "-low"
	default:
		return m.name + "-default"
	}
}

func (m *mockProviderWithUsageLimitTracking) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithUsageLimitTracking) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderWithUsageLimitTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.runValidationResult != nil {
		return m.runValidationResult, nil
	}
	return &provider.Result{Success: true, Model: "mock-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderWithUsageLimitTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.isUsageLimitErrorFn != nil {
		return m.isUsageLimitErrorFn(result, err)
	}
	return false
}

// mockClaudeClientForProcess provides backward-compatible claude client for tests.
// This allows tests to run against current implementation (which uses r.claude)
// without panicking, while still testing the new router behavior.
// Tests will pass using this mock but should fail their assertions because
// router.Select() wasn't called.
type mockClaudeClientForProcess struct{}

func (m *mockClaudeClientForProcess) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	return &claude.Result{Success: true, Model: model, Output: "done"}, nil
}

func (m *mockClaudeClientForProcess) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	// Current implementation uses this path - tests should fail because router.Select() wasn't called
	return &claude.Result{Success: true, Model: model, Output: "done"}, nil
}

func (m *mockClaudeClientForProcess) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	return &claude.Result{Success: true, Model: model}, nil
}

// mockStateForRouterTest provides a minimal StateFile implementation for router tests
type mockStateForRouterTest struct {
	counts           map[string]int
	onSetUnavailable func(string, time.Time)
}

func (m *mockStateForRouterTest) IncrementProviderCount(provider string) {
	if m.counts == nil {
		m.counts = make(map[string]int)
	}
	m.counts[provider]++
}

func (m *mockStateForRouterTest) GetProviderCounts() map[string]int {
	if m.counts == nil {
		return make(map[string]int)
	}
	return m.counts
}

func (m *mockStateForRouterTest) IsProviderAvailable(provider string) bool {
	return true
}

func (m *mockStateForRouterTest) SetProviderUnavailable(provider string, until time.Time) {
	if m.onSetUnavailable != nil {
		m.onSetUnavailable(provider, until)
	}
}

// TestAcceptance_RunRefactorPhaseUsesRouter verifies runRefactorPhase uses router
func TestAcceptance_RunRefactorPhaseUsesRouter(t *testing.T) {
	tmpDir, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled, cfg.Validation.Commands = true, []string{"go test"}

	var capturedTier string
	mockProvider := &mockProviderWithSelectTracking{
		runFn: func(_ context.Context, _ string, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{Success: true, Model: "test-model", Output: "done"}, nil
		},
		runValidationResult: &provider.Result{Success: true, Model: "vm", Output: "VALIDATION_PASSED"},
	}

	bc := &beadContext{
		bead:        &bead.Bead{ID: "r001", Priority: 1},
		tier:        provider.TierHigh,
		result:      &IterationResult{},
		promptCtx:   &prompt.Context{WorkDir: tmpDir},
		startCommit: startCommit,
	}

	r := &Runner{
		cfg:    cfg,
		router: provider.NewSingleProviderRouter(mockProvider),
		renderer: &mockPromptRenderer{
			RenderRefactorFn: func(*prompt.Context) (string, error) { return "refactor", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff", nil },
	}

	if err := r.runRefactorPhase(context.Background(), bc); err != nil {
		t.Fatalf("error = %v", err)
	}

	if capturedTier != bc.tier {
		t.Errorf("tier = %q, want %q", capturedTier, bc.tier)
	}
}

func setupTestGitRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
		return string(out)
	}
	run("git", "init")
	run("git", "config", "user.email", "t@t.com")
	run("git", "config", "user.name", "T")
	os.WriteFile(filepath.Join(dir, "t.txt"), []byte("v1"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-m", "c1")
	start := strings.TrimSpace(run("git", "rev-parse", "HEAD"))
	os.WriteFile(filepath.Join(dir, "t.txt"), []byte("v2"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-m", "c2")
	return dir, start
}

// Review router conversion tests

// TestAcceptance_RunLightReviewUsesRouter verifies that runLightReview
// calls router.Select() with phase="review" and the tier from selectReviewTier().
func TestAcceptance_RunLightReviewUsesRouter(t *testing.T) {
	_, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()

	var capturedTier string
	providerCalled := false

	mockProvider := &mockProviderWithSelectTracking{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerCalled = true
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
		ID:       "review-001",
		Priority: 1,
		Labels:   []string{},
	}

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
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

	_, err := r.runLightReview(context.Background(), b, nil, startCommit, "sonnet", 1, time.Time{})
	if err != nil || !providerCalled {
		t.Fatalf("error = %v, providerCalled = %v", err, providerCalled)
	}

	if capturedTier != r.selectTier(b) {
		t.Errorf("tier = %q, want %q", capturedTier, r.selectTier(b))
	}
}

// TestAcceptance_RunLightReviewUsesHighTierForOpusBuild verifies that when
// buildModel is "opus" and ShouldMatchBuildModel is true, runLightReview uses tier "high".
func TestAcceptance_RunLightReviewUsesHighTierForOpusBuild(t *testing.T) {
	_, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	trueVal := true
	cfg.Review.MatchBuildModel = &trueVal

	var capturedTier string

	mockProvider := &mockProviderWithSelectTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Model:   "test-opus",
				Output:  `{"passed": true, "summary": "LGTM: OK", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	b := &bead.Bead{ID: "review-003", Priority: 2}

	r := &Runner{
		cfg:    cfg,
		router: provider.NewSingleProviderRouter(mockProvider),
		renderer: &mockPromptRenderer{
			RenderReviewFn: func(*prompt.ReviewContext) (string, error) { return "review", nil },
			LoadClaudeMDFn: func() (string, error) { return "", nil },
			LoadRulesFn:    func() (string, error) { return "", nil },
		},
		output:    io.Discard,
		gitDiffFn: func(string) (string, error) { return "diff", nil },
	}

	_, err := r.runLightReview(context.Background(), b, nil, startCommit, "opus", 1, time.Time{})
	if err != nil || capturedTier != provider.TierHigh {
		t.Errorf("error = %v, tier = %q, want %q", err, capturedTier, provider.TierHigh)
	}
}

// TestAcceptance_RunThoroughReviewUsesRouter verifies that runThoroughReview
// calls router.Select() with phase="review" and tier="high".
func TestAcceptance_RunThoroughReviewUsesRouter(t *testing.T) {
	tmpDir, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = false

	var capturedTier string

	mockProvider := &mockProviderWithSelectTracking{
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

	stateDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(stateDir, 0755)
	sf, _ := state.NewFile(stateDir)
	sf.RecordReview(startCommit, 0)

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
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

	if capturedTier != provider.TierHigh {
		t.Errorf("tier = %q, want %q", capturedTier, provider.TierHigh)
	}
}

// TestAcceptance_RunThoroughReviewValidationUsesRouter verifies that
// the validation sub-call in runThoroughReview uses router with tier="low".
func TestAcceptance_RunThoroughReviewValidationUsesRouter(t *testing.T) {
	tmpDir, startCommit := setupTestGitRepo(t)
	cfg := makeTestRunnerConfig()
	cfg.Validation.Enabled = true
	cfg.Validation.Commands = []string{"echo test"}

	var validationTier string
	validationCalled := false

	mockProvider := &mockProviderWithValidationTracking{
		mockProviderWithSelectTracking: &mockProviderWithSelectTracking{
			runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
				return &provider.Result{
					Success: true,
					Model:   "test-opus",
					Output:  `{"passed": true, "summary": "LGTM: Fixed", "fixes_applied": ["test.go"], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
				}, nil
			},
		},
		onValidationSelect: func(phase, tier string) {
			validationTier = tier
			validationCalled = true
		},
		runValidationResult: &provider.Result{Success: true, Model: "test-haiku", Output: "VALIDATION_PASSED"},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	stateDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(stateDir, 0755)
	sf, _ := state.NewFile(stateDir)
	sf.RecordReview(startCommit, 0)

	r := &Runner{
		cfg:    cfg,
		router: mockRouter,
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

	if !validationCalled || validationTier != provider.TierLow {
		t.Errorf("validationCalled = %v, tier = %q, want %q", validationCalled, validationTier, provider.TierLow)
	}
}

// mockProviderWithValidationTracking extends mockProviderWithSelectTracking
// to track validation calls separately
type mockProviderWithValidationTracking struct {
	*mockProviderWithSelectTracking
	onValidationSelect  func(phase, tier string)
	runValidationResult *provider.Result
	runValidationErr    error
}

func (m *mockProviderWithValidationTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.onValidationSelect != nil {
		m.onValidationSelect("validate", tier)
	}
	if m.runValidationResult != nil {
		return m.runValidationResult, m.runValidationErr
	}
	return &provider.Result{Success: true, Model: "test-haiku", Output: "VALIDATION_PASSED"}, nil
}

// mockClaudeClientForReview provides backward-compatible claude client for review tests.
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
