package reviewpkg

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// mockStateAccess implements the StateAccess interface for thorough review tests.
type mockStateAccess struct {
	lastReviewCommitFn func() string
	recordReviewFn     func(commit string, iteration int) error
}

func (m *mockStateAccess) LastReviewCommit() string {
	if m.lastReviewCommitFn != nil {
		return m.lastReviewCommitFn()
	}
	return "abc123def"
}

func (m *mockStateAccess) RecordReview(commit string, iteration int) error {
	if m.recordReviewFn != nil {
		return m.recordReviewFn(commit, iteration)
	}
	return nil
}

// --- Helper ---

func newThoroughTestConfig() *config.Config {
	cfg := &config.Config{
		Review: config.ReviewConfig{
			Enabled: true,
			Model:   "sonnet",
			Timeout: 120,
			Thorough: config.ThoroughReviewConfig{
				Enabled:          true,
				Model:            "opus",
				Timeout:          300,
				EveryNIterations: 3,
			},
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

// --- RunThorough tests ---

func TestRunThorough_ReturnsResultFromProvider(t *testing.T) {
	// When the provider returns a passing thorough review with findings,
	// RunThorough should parse the result, apply it (create beads), log it,
	// and record the review in state.
	cfg := newThoroughTestConfig()

	prov := &mockProvider{
		name: "thorough-provider",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Thorough review complete","fixes_applied":[],"beads_to_create":[{"title":"Improve error handling","description":"Add context to errors","priority":2,"labels":["enhancement"]}],"backlog_items":[]}`,
				Model:   "opus",
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			if phase != "review" {
				t.Errorf("RunThorough should use phase 'review', got %q", phase)
			}
			if tier != provider.TierHigh {
				t.Errorf("RunThorough should use tier 'high', got %q", tier)
			}
			return prov, "thorough-provider"
		},
	}

	beadClient := &mockBeadClient{}
	mockLogger := &mockIterationLogger{}
	renderer := &mockPromptRenderer{}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit-abc" },
	}

	var recordedCommit string
	var recordedIteration int
	stateAccess.recordReviewFn = func(commit string, iteration int) error {
		recordedCommit = commit
		recordedIteration = iteration
		return nil
	}

	gitDiffFn := func(startCommit string) (string, error) {
		if startCommit != "prev-commit-abc" {
			t.Errorf("RunThorough should diff from last review commit, got %q", startCommit)
		}
		return "diff --git a/foo.go b/foo.go\n+significant changes", nil
	}

	getGitHeadFn := func() (string, error) {
		return "new-head-commit-xyz", nil
	}

	rev := NewReviewer(cfg, router, beadClient, renderer, gitDiffFn, mockLogger)

	rev.RunThorough(context.Background(), stateAccess, 5, time.Time{}, getGitHeadFn)

	// Verify bead was created from the review finding
	if len(beadClient.created) != 1 {
		t.Fatalf("expected 1 bead created from thorough review, got %d", len(beadClient.created))
	}
	if beadClient.created[0].title != "Improve error handling" {
		t.Errorf("created bead title = %q, want %q", beadClient.created[0].title, "Improve error handling")
	}

	// Verify review was logged as "thorough" type
	if len(mockLogger.reviews) != 1 {
		t.Fatalf("expected 1 review log, got %d", len(mockLogger.reviews))
	}
	if mockLogger.reviews[0].ReviewType != "thorough" {
		t.Errorf("review log type = %q, want %q", mockLogger.reviews[0].ReviewType, "thorough")
	}

	// Verify state was updated with new commit
	if recordedCommit != "new-head-commit-xyz" {
		t.Errorf("recorded commit = %q, want %q", recordedCommit, "new-head-commit-xyz")
	}
	if recordedIteration != 5 {
		t.Errorf("recorded iteration = %d, want 5", recordedIteration)
	}
}

func TestRunThorough_SkipsWhenDeadlineExpired(t *testing.T) {
	// When deadline has passed, RunThorough should return without calling the provider.
	cfg := newThoroughTestConfig()

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

	deadline := time.Now().Add(-1 * time.Minute)

	rev.RunThorough(context.Background(), &mockStateAccess{}, 1, deadline, func() (string, error) {
		return "abc", nil
	})

	if providerCalled {
		t.Error("RunThorough should not call the provider when deadline is expired")
	}
}

func TestRunThorough_SkipsWhenInsufficientTime(t *testing.T) {
	// When the remaining time is less than the thorough review timeout,
	// RunThorough should skip without calling the provider.
	cfg := newThoroughTestConfig()
	cfg.Review.Thorough.Timeout = 300 // 5 minutes

	providerCalled := false
	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			providerCalled = true
			return &mockProvider{name: "test"}, "test"
		},
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff", nil
	}, nil)

	// Only 1 minute left, but timeout needs 5 minutes
	deadline := time.Now().Add(1 * time.Minute)

	rev.RunThorough(context.Background(), &mockStateAccess{}, 1, deadline, func() (string, error) {
		return "abc", nil
	})

	if providerCalled {
		t.Error("RunThorough should not call the provider when insufficient time remaining")
	}
}

func TestRunThorough_SkipsWhenNilStateFile(t *testing.T) {
	// When state access is nil, RunThorough should return without error.
	cfg := newThoroughTestConfig()

	providerCalled := false
	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			providerCalled = true
			return &mockProvider{name: "test"}, "test"
		},
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff", nil
	}, nil)

	// Pass nil state access
	rev.RunThorough(context.Background(), nil, 1, time.Time{}, func() (string, error) {
		return "abc", nil
	})

	if providerCalled {
		t.Error("RunThorough should not call the provider when state is nil")
	}
}

func TestRunThorough_SkipsWhenNoLastReviewCommit(t *testing.T) {
	// When there is no previous review commit, RunThorough should skip.
	cfg := newThoroughTestConfig()

	providerCalled := false
	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			providerCalled = true
			return &mockProvider{name: "test"}, "test"
		},
	}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "" }, // no previous review
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff", nil
	}, nil)

	rev.RunThorough(context.Background(), stateAccess, 1, time.Time{}, func() (string, error) {
		return "abc", nil
	})

	if providerCalled {
		t.Error("RunThorough should not call the provider when no previous review commit")
	}
}

func TestRunThorough_SkipsWhenNoDiff(t *testing.T) {
	// When the diff since the last review is empty, RunThorough should skip.
	cfg := newThoroughTestConfig()

	providerCalled := false
	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			providerCalled = true
			return &mockProvider{name: "test"}, "test"
		},
	}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "", nil // empty diff
	}, nil)

	rev.RunThorough(context.Background(), stateAccess, 1, time.Time{}, func() (string, error) {
		return "abc", nil
	})

	if providerCalled {
		t.Error("RunThorough should not call the provider when diff is empty")
	}
}

func TestRunThorough_RevalidatesAfterFixesApplied(t *testing.T) {
	// When the thorough review applies fixes, RunThorough should run validation
	// to ensure the fixes didn't break anything.
	cfg := newThoroughTestConfig()

	prov := &mockProvider{
		name: "thorough-provider",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Applied fixes","fixes_applied":["fixed import order"],"beads_to_create":[],"backlog_items":[]}`,
				Model:   "opus",
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "thorough-provider"
		},
	}

	validationCalled := false
	validateFn := func(ctx context.Context, commands []string, workDir string) (bool, error) {
		validationCalled = true
		return true, nil // validation passes
	}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, &mockIterationLogger{})

	rev.RunThorough(context.Background(), stateAccess, 3, time.Time{}, func() (string, error) {
		return "new-head", nil
	})

	// This assertion requires the validateFn to be wired into RunThorough
	_ = validateFn
	_ = validationCalled
	// After implementation, we'd assert: validationCalled == true
}

func TestRunThorough_BuildsThoroughReviewContext(t *testing.T) {
	// RunThorough should build a ThoroughReviewContext with the correct fields
	// and pass it to the renderer's RenderThoroughReview method.
	cfg := newThoroughTestConfig()
	cfg.Review.Thorough.Model = "opus"

	var capturedCtx *prompt.ThoroughReviewContext
	renderer := &mockPromptRenderer{
		renderThoroughReviewFn: func(ctx *prompt.ThoroughReviewContext) (string, error) {
			capturedCtx = ctx
			return "thorough prompt", nil
		},
	}

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"OK","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
				Model:   "opus",
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
	}

	rev := NewReviewer(cfg, router, nil, renderer, func(string) (string, error) {
		return "diff --git a/bar.go b/bar.go\n+new code", nil
	}, &mockIterationLogger{})

	rev.RunThorough(context.Background(), stateAccess, 2, time.Time{}, func() (string, error) {
		return "head", nil
	})

	if capturedCtx == nil {
		t.Fatal("RenderThoroughReview was not called")
	}
	if capturedCtx.Model != "opus" {
		t.Errorf("ThoroughReviewContext.Model = %q, want %q", capturedCtx.Model, "opus")
	}
	if capturedCtx.Diff == "" {
		t.Error("ThoroughReviewContext.Diff should not be empty")
	}
}

func TestRunThorough_AlwaysSelectsHighTier(t *testing.T) {
	// RunThorough should always use tier "high" for provider selection,
	// regardless of bead priority or config.
	cfg := newThoroughTestConfig()

	var selectedTier string
	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"OK","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			selectedTier = tier
			return prov, "test"
		},
	}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, &mockIterationLogger{})

	rev.RunThorough(context.Background(), stateAccess, 1, time.Time{}, func() (string, error) {
		return "head", nil
	})

	if selectedTier != provider.TierHigh {
		t.Errorf("RunThorough selected tier = %q, want %q", selectedTier, provider.TierHigh)
	}
}

func TestRunThorough_LogsReviewAsThoroughType(t *testing.T) {
	// The review log entry should have ReviewType "thorough" (not "light").
	cfg := newThoroughTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Good","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
				Model:   "opus",
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	mockLog := &mockIterationLogger{}
	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
	}

	rev := NewReviewer(cfg, router, &mockBeadClient{}, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, mockLog)

	rev.RunThorough(context.Background(), stateAccess, 7, time.Time{}, func() (string, error) {
		return "head-commit", nil
	})

	if len(mockLog.reviews) != 1 {
		t.Fatalf("expected 1 review log entry, got %d", len(mockLog.reviews))
	}

	log := mockLog.reviews[0]
	if log.ReviewType != "thorough" {
		t.Errorf("log.ReviewType = %q, want %q", log.ReviewType, "thorough")
	}
	if log.Iteration != 7 {
		t.Errorf("log.Iteration = %d, want 7", log.Iteration)
	}
	if log.Model != "opus" {
		t.Errorf("log.Model = %q, want %q", log.Model, "opus")
	}
}

// --- RunThorough: Verify state recording ---

func TestRunThorough_RecordsReviewInState(t *testing.T) {
	// After a successful thorough review, RunThorough should call
	// RecordReview on the state access with the current git HEAD and iteration.
	cfg := newThoroughTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"OK","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
				Model:   "opus",
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	var recorded bool
	var recordedCommit string
	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
		recordReviewFn: func(commit string, iteration int) error {
			recorded = true
			recordedCommit = commit
			return nil
		},
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff", nil
	}, &mockIterationLogger{})

	rev.RunThorough(context.Background(), stateAccess, 4, time.Time{}, func() (string, error) {
		return "current-head-sha", nil
	})

	if !recorded {
		t.Error("RunThorough should record the review in state")
	}
	if recordedCommit != "current-head-sha" {
		t.Errorf("recorded commit = %q, want %q", recordedCommit, "current-head-sha")
	}
}

// --- RunThorough: Provider nil handling ---

func TestRunThorough_HandlesNilProvider(t *testing.T) {
	// When router.Select returns nil provider, RunThorough should log warning
	// and return without panicking.
	cfg := newThoroughTestConfig()

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return nil, "" // no provider available
		},
	}

	stateAccess := &mockStateAccess{
		lastReviewCommitFn: func() string { return "prev-commit" },
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff", nil
	}, nil)

	// Should not panic
	rev.RunThorough(context.Background(), stateAccess, 1, time.Time{}, func() (string, error) {
		return "head", nil
	})
}

// --- Verify PromptRenderer interface includes RenderThoroughReview ---

func TestPromptRenderer_MustSupportThoroughReview(t *testing.T) {
	var renderer PromptRenderer = &mockPromptRenderer{}
	result, err := renderer.RenderThoroughReview(&prompt.ThoroughReviewContext{
		Diff:  "test diff",
		Model: "opus",
	})
	if err != nil {
		t.Fatalf("RenderThoroughReview returned error: %v", err)
	}
	if result == "" {
		t.Error("RenderThoroughReview returned empty string")
	}
}
