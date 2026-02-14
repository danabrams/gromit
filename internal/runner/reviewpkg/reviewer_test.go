package reviewpkg

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/runner/escalation"
)

// --- Mock types for narrow interfaces ---

// mockRouter implements the Router interface for reviewpkg tests.
type mockRouter struct {
	selectFn      func(phase, tier string) (provider.Provider, string)
	selectCrossFn func(buildProvider, tier string) (provider.Provider, string)
}

func (m *mockRouter) Select(phase, tier string) (provider.Provider, string) {
	if m.selectFn != nil {
		return m.selectFn(phase, tier)
	}
	return nil, ""
}

func (m *mockRouter) SelectCross(buildProvider, tier string) (provider.Provider, string) {
	if m.selectCrossFn != nil {
		return m.selectCrossFn(buildProvider, tier)
	}
	return nil, ""
}

// mockBeadClient implements the BeadClient interface for reviewpkg tests.
type mockBeadClient struct {
	createFn func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
	created  []createdBead
}

type createdBead struct {
	title       string
	priority    int
	labels      []string
	parentID    string
	description string
}

func (m *mockBeadClient) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	m.created = append(m.created, createdBead{title: title, priority: priority, labels: labels, parentID: parentID, description: description})
	if m.createFn != nil {
		return m.createFn(title, priority, labels, expectedOutputs, parentID, description)
	}
	return &bead.Bead{ID: fmt.Sprintf("created-%d", len(m.created)), Title: title}, nil
}

// mockPromptRenderer implements the PromptRenderer interface for reviewpkg tests.
type mockPromptRenderer struct {
	renderReviewFn         func(ctx *prompt.ReviewContext) (string, error)
	renderThoroughReviewFn func(ctx *prompt.ThoroughReviewContext) (string, error)
	loadClaudeMDFn         func() (string, error)
	loadRulesFn            func(phase string) (string, error)
	loadSpecFn             func(name string) (string, error)
}

func (m *mockPromptRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	if m.renderReviewFn != nil {
		return m.renderReviewFn(ctx)
	}
	return "review prompt", nil
}

func (m *mockPromptRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	if m.renderThoroughReviewFn != nil {
		return m.renderThoroughReviewFn(ctx)
	}
	return "thorough review prompt", nil
}

func (m *mockPromptRenderer) LoadClaudeMD() (string, error) {
	if m.loadClaudeMDFn != nil {
		return m.loadClaudeMDFn()
	}
	return "# CLAUDE.md", nil
}

func (m *mockPromptRenderer) LoadRulesForPhase(phase string) (string, error) {
	if m.loadRulesFn != nil {
		return m.loadRulesFn(phase)
	}
	return "# Rules", nil
}

func (m *mockPromptRenderer) LoadSpec(name string) (string, error) {
	if m.loadSpecFn != nil {
		return m.loadSpecFn(name)
	}
	return "# Spec", nil
}

// mockProvider implements provider.Provider for reviewpkg tests.
type mockProvider struct {
	name  string
	runFn func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "opus"
	case provider.TierMedium:
		return "sonnet"
	default:
		return "haiku"
	}
}
func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Output: `{"passed":true,"summary":"LGTM","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`}, nil
}
func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return nil, nil
}
func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return nil, nil
}
func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (m *mockProvider) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}
func (m *mockProvider) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

// mockIterationLogger implements the IterationLogger interface for reviewpkg tests.
type mockIterationLogger struct {
	reviews []*logger.ReviewLog
}

func (m *mockIterationLogger) LogReview(log *logger.ReviewLog) error {
	m.reviews = append(m.reviews, log)
	return nil
}

// --- Helper functions ---

func newTestConfig() *config.Config {
	cfg := &config.Config{
		Review: config.ReviewConfig{
			Enabled: true,
			Model:   "sonnet",
			Timeout: 120,
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./...", "go vet ./..."},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// --- RunLight tests ---

func TestRunLight_ReturnsPassingReview(t *testing.T) {
	// When the provider returns a passing review, RunLight should return
	// a ReviewResult with Passed=true and the summary from the output.
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"All looks good","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test-provider"
		},
	}

	renderer := &mockPromptRenderer{}
	gitDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/foo.go b/foo.go\n+added line", nil
	}

	rev := NewReviewer(cfg, router, nil, renderer, gitDiffFn, nil)

	b := &bead.Bead{ID: "test-001", Title: "Test bead", Priority: 1}
	result, err := rev.RunLight(context.Background(), b, nil, "abc123", "sonnet", 1, time.Time{}, "test-provider")
	if err != nil {
		t.Fatalf("RunLight returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("RunLight returned nil result")
	}
	if !result.Passed {
		t.Error("RunLight result.Passed = false, want true")
	}
	if result.Summary != "All looks good" {
		t.Errorf("RunLight result.Summary = %q, want %q", result.Summary, "All looks good")
	}
}

func TestRunLight_SkipsWhenDeadlineExpired(t *testing.T) {
	// When the deadline has already passed, RunLight should return nil, nil
	// without invoking the provider.
	cfg := newTestConfig()
	providerCalled := false

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			providerCalled = true
			return &mockProvider{name: "test"}, "test"
		},
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "some diff", nil
	}, nil)

	b := &bead.Bead{ID: "test-002", Priority: 1}
	// Deadline in the past
	deadline := time.Now().Add(-1 * time.Minute)

	result, err := rev.RunLight(context.Background(), b, nil, "abc123", "sonnet", 1, deadline, "")
	if err != nil {
		t.Fatalf("RunLight returned unexpected error: %v", err)
	}
	if result != nil {
		t.Error("RunLight should return nil result when deadline is expired")
	}
	if providerCalled {
		t.Error("RunLight should not call the provider when deadline is expired")
	}
}

func TestRunLight_SkipsWhenNoDiff(t *testing.T) {
	// When the git diff returns empty, RunLight should return nil, nil
	// without invoking the provider.
	cfg := newTestConfig()

	rev := NewReviewer(cfg, &mockRouter{}, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "", nil // no diff
	}, nil)

	b := &bead.Bead{ID: "test-003", Priority: 1}
	result, err := rev.RunLight(context.Background(), b, nil, "abc123", "sonnet", 1, time.Time{}, "")
	if err != nil {
		t.Fatalf("RunLight returned unexpected error: %v", err)
	}
	if result != nil {
		t.Error("RunLight should return nil result when there is no diff")
	}
}

func TestRunLight_UsesCrossReviewWhenConfigured(t *testing.T) {
	// When routing.phase_preferences["review"] == "cross" and a buildProvider
	// is specified, RunLight should call SelectCross instead of Select.
	cfg := newTestConfig()
	cfg.Routing.PhasePreferences["review"] = "cross"

	var usedCrossReview bool
	prov := &mockProvider{
		name: "cross-provider",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Cross review OK","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "build-provider" // should NOT be called
		},
		selectCrossFn: func(buildProvider, tier string) (provider.Provider, string) {
			usedCrossReview = true
			if buildProvider != "build-provider" {
				t.Errorf("SelectCross buildProvider = %q, want %q", buildProvider, "build-provider")
			}
			return prov, "cross-provider"
		},
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, nil)

	b := &bead.Bead{ID: "test-004", Priority: 1}
	_, err := rev.RunLight(context.Background(), b, nil, "abc123", "sonnet", 1, time.Time{}, "build-provider")
	if err != nil {
		t.Fatalf("RunLight returned unexpected error: %v", err)
	}
	if !usedCrossReview {
		t.Error("RunLight should use SelectCross when review phase preference is 'cross' and buildProvider is set")
	}
}

func TestRunLight_LoadsSpecFromBeadOrParentLabels(t *testing.T) {
	// When the bead or parent has a spec label, RunLight should load the spec
	// and include it in the ReviewContext passed to the renderer.
	cfg := newTestConfig()
	prov := &mockProvider{name: "test"}

	var capturedCtx *prompt.ReviewContext
	renderer := &mockPromptRenderer{
		renderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			capturedCtx = ctx
			return "review prompt", nil
		},
		loadSpecFn: func(name string) (string, error) {
			if name != "my-spec" {
				t.Errorf("LoadSpec name = %q, want %q", name, "my-spec")
			}
			return "# My Spec Content", nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	rev := NewReviewer(cfg, router, nil, renderer, func(string) (string, error) {
		return "diff content", nil
	}, nil)

	// Parent has the spec label, bead does not
	b := &bead.Bead{ID: "test-005", Priority: 1, Labels: []string{}}
	parent := &bead.Bead{ID: "parent-001", Labels: []string{"spec:my-spec"}}

	_, err := rev.RunLight(context.Background(), b, parent, "abc123", "sonnet", 1, time.Time{}, "")
	if err != nil {
		t.Fatalf("RunLight returned unexpected error: %v", err)
	}
	if capturedCtx == nil {
		t.Fatal("renderer.RenderReview was not called")
	}
	if capturedCtx.Spec != "# My Spec Content" {
		t.Errorf("ReviewContext.Spec = %q, want %q", capturedCtx.Spec, "# My Spec Content")
	}
}

// --- SelectReviewTier tests ---

func TestSelectReviewTier_OpusBuildReturnsHigh(t *testing.T) {
	// When buildModel is "opus", SelectReviewTier should always return "high"
	// regardless of the bead's priority or labels.
	cfg := newTestConfig()
	b := &bead.Bead{ID: "test-tier-001", Priority: 2} // P2 would normally be low

	tier := SelectReviewTier(cfg, b, "opus")
	if tier != provider.TierHigh {
		t.Errorf("SelectReviewTier(opus) = %q, want %q", tier, provider.TierHigh)
	}
}

func TestSelectReviewTier_NonOpusDelegatesToEscalation(t *testing.T) {
	// When buildModel is not "opus", SelectReviewTier should delegate to
	// escalation.SelectTier based on the bead's priority and labels.
	cfg := newTestConfig()
	cfg.Models.P1 = provider.TierMedium

	b := &bead.Bead{ID: "test-tier-002", Priority: 1}

	tier := SelectReviewTier(cfg, b, "sonnet")
	// escalation.SelectTier for P1 with cfg.Models.P1 = "medium" should return "medium"
	expected := escalation.SelectTier(cfg, b)
	if tier != expected {
		t.Errorf("SelectReviewTier(sonnet) = %q, want %q (from escalation.SelectTier)", tier, expected)
	}
}

// --- ApplyResult tests ---

func TestApplyResult_CreatesBeadsFromProposals(t *testing.T) {
	// When a ReviewResult has BeadsToCreate, ApplyResult should create them
	// with the "from-review" label prepended, and return the count.
	cfg := newTestConfig()
	beadClient := &mockBeadClient{}

	rev := NewReviewer(cfg, nil, beadClient, nil, nil, nil)

	result := &review.ReviewResult{
		Passed: true,
		BeadsToCreate: []review.BeadProposal{
			{Title: "Fix error handling", Description: "Add nil check", Priority: 1, Labels: []string{"bug"}},
			{Title: "Add logging", Description: "Structured logs", Priority: 2, Labels: []string{"enhancement"}},
		},
		BacklogItems: []review.BacklogItem{},
	}

	beadsCreated, backlogCreated := rev.ApplyResult(result)

	if beadsCreated != 2 {
		t.Errorf("ApplyResult beadsCreated = %d, want 2", beadsCreated)
	}
	if backlogCreated != 0 {
		t.Errorf("ApplyResult backlogCreated = %d, want 0", backlogCreated)
	}
	if len(beadClient.created) != 2 {
		t.Fatalf("beadClient.created has %d entries, want 2", len(beadClient.created))
	}

	// First bead should have "from-review" + "bug" labels
	first := beadClient.created[0]
	if first.title != "Fix error handling" {
		t.Errorf("first bead title = %q, want %q", first.title, "Fix error handling")
	}
	if first.priority != 1 {
		t.Errorf("first bead priority = %d, want 1", first.priority)
	}
	if len(first.labels) < 2 || first.labels[0] != "from-review" || first.labels[1] != "bug" {
		t.Errorf("first bead labels = %v, want [from-review bug]", first.labels)
	}
}

func TestApplyResult_CreatesBacklogItemsAsP2(t *testing.T) {
	// BacklogItems should be created as P2 beads with both "from-review"
	// and "backlog" labels, combining description and reason.
	cfg := newTestConfig()
	beadClient := &mockBeadClient{}

	rev := NewReviewer(cfg, nil, beadClient, nil, nil, nil)

	result := &review.ReviewResult{
		Passed:        true,
		BeadsToCreate: []review.BeadProposal{},
		BacklogItems: []review.BacklogItem{
			{Title: "Refactor auth", Description: "Extract interface", Reason: "Improves testability"},
		},
	}

	beadsCreated, backlogCreated := rev.ApplyResult(result)

	if beadsCreated != 0 {
		t.Errorf("ApplyResult beadsCreated = %d, want 0", beadsCreated)
	}
	if backlogCreated != 1 {
		t.Errorf("ApplyResult backlogCreated = %d, want 1", backlogCreated)
	}
	if len(beadClient.created) != 1 {
		t.Fatalf("beadClient.created has %d entries, want 1", len(beadClient.created))
	}

	item := beadClient.created[0]
	if item.priority != 2 {
		t.Errorf("backlog item priority = %d, want 2", item.priority)
	}
	if len(item.labels) < 2 || item.labels[0] != "from-review" || item.labels[1] != "backlog" {
		t.Errorf("backlog item labels = %v, want [from-review backlog]", item.labels)
	}
	// Description should combine description and reason
	if item.description == "" {
		t.Error("backlog item description is empty, should combine description and reason")
	}
	wantDesc := "Extract interface\n\nReason for backlog: Improves testability"
	if item.description != wantDesc {
		t.Errorf("backlog item description = %q, want %q", item.description, wantDesc)
	}
}

func TestApplyResult_NilResultReturnsZero(t *testing.T) {
	// When result is nil, ApplyResult should return 0, 0 without panicking.
	cfg := newTestConfig()
	rev := NewReviewer(cfg, nil, &mockBeadClient{}, nil, nil, nil)

	beadsCreated, backlogCreated := rev.ApplyResult(nil)
	if beadsCreated != 0 || backlogCreated != 0 {
		t.Errorf("ApplyResult(nil) = (%d, %d), want (0, 0)", beadsCreated, backlogCreated)
	}
}

func TestApplyResult_DeduplicatesFromReviewLabel(t *testing.T) {
	// When a proposal already has "from-review" in its labels, ApplyResult
	// should not duplicate it.
	cfg := newTestConfig()
	beadClient := &mockBeadClient{}

	rev := NewReviewer(cfg, nil, beadClient, nil, nil, nil)

	result := &review.ReviewResult{
		Passed: true,
		BeadsToCreate: []review.BeadProposal{
			{Title: "Fix it", Priority: 1, Labels: []string{"from-review", "bug"}},
		},
	}

	rev.ApplyResult(result)

	if len(beadClient.created) != 1 {
		t.Fatalf("beadClient.created has %d entries, want 1", len(beadClient.created))
	}
	labels := beadClient.created[0].labels
	count := 0
	for _, l := range labels {
		if l == "from-review" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("'from-review' appears %d times in labels %v, want exactly 1", count, labels)
	}
}

func TestApplyResult_ContinuesOnCreateError(t *testing.T) {
	// When creating one bead fails, ApplyResult should continue creating
	// the remaining beads and not count the failed one.
	cfg := newTestConfig()
	callCount := 0
	beadClient := &mockBeadClient{
		createFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			callCount++
			if callCount == 1 {
				return nil, fmt.Errorf("creation failed")
			}
			return &bead.Bead{ID: "ok"}, nil
		},
	}

	rev := NewReviewer(cfg, nil, beadClient, nil, nil, nil)

	result := &review.ReviewResult{
		BeadsToCreate: []review.BeadProposal{
			{Title: "Fails", Priority: 1},
			{Title: "Succeeds", Priority: 1},
		},
	}

	beadsCreated, _ := rev.ApplyResult(result)
	if beadsCreated != 1 {
		t.Errorf("ApplyResult beadsCreated = %d, want 1 (first should fail, second should succeed)", beadsCreated)
	}
}

// --- WriteReviewLog tests ---

func TestWriteReviewLog_LogsCorrectFields(t *testing.T) {
	// WriteReviewLog should call the IterationLogger with a ReviewLog
	// containing all the correct field values.
	cfg := newTestConfig()
	mockLogger := &mockIterationLogger{}

	rev := NewReviewer(cfg, nil, nil, nil, nil, mockLogger)

	result := &review.ReviewResult{
		Passed:       true,
		FixesApplied: []string{"fixed import", "fixed format"},
	}

	rev.WriteReviewLog(5, "bead-123", "sonnet", result, 2, 1, 3*time.Second)

	if len(mockLogger.reviews) != 1 {
		t.Fatalf("mockLogger.reviews has %d entries, want 1", len(mockLogger.reviews))
	}
	log := mockLogger.reviews[0]
	if log.Type != "review" {
		t.Errorf("log.Type = %q, want %q", log.Type, "review")
	}
	if log.ReviewType != "light" {
		t.Errorf("log.ReviewType = %q, want %q", log.ReviewType, "light")
	}
	if log.Iteration != 5 {
		t.Errorf("log.Iteration = %d, want 5", log.Iteration)
	}
	if log.BeadID != "bead-123" {
		t.Errorf("log.BeadID = %q, want %q", log.BeadID, "bead-123")
	}
	if log.Model != "sonnet" {
		t.Errorf("log.Model = %q, want %q", log.Model, "sonnet")
	}
	if !log.Passed {
		t.Error("log.Passed = false, want true")
	}
	if log.FixesApplied != 2 {
		t.Errorf("log.FixesApplied = %d, want 2", log.FixesApplied)
	}
	if log.BeadsCreated != 2 {
		t.Errorf("log.BeadsCreated = %d, want 2", log.BeadsCreated)
	}
	if log.BacklogCreated != 1 {
		t.Errorf("log.BacklogCreated = %d, want 1", log.BacklogCreated)
	}
	if log.DurationMs != 3000 {
		t.Errorf("log.DurationMs = %d, want 3000", log.DurationMs)
	}
}

func TestWriteReviewLog_NilResultIsNoOp(t *testing.T) {
	// When result is nil, WriteReviewLog should not panic and should not log.
	cfg := newTestConfig()
	mockLogger := &mockIterationLogger{}

	rev := NewReviewer(cfg, nil, nil, nil, nil, mockLogger)

	rev.WriteReviewLog(1, "bead-nil", "sonnet", nil, 0, 0, time.Second)

	if len(mockLogger.reviews) != 0 {
		t.Errorf("mockLogger.reviews has %d entries, want 0 (nil result should be no-op)", len(mockLogger.reviews))
	}
}
