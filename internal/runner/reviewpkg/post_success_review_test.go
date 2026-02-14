package reviewpkg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Helpers for RunPostSuccess tests ---

func newTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:          &bead.Bead{ID: "test-bead-001", Title: "Test bead", Priority: 1},
		Parent:        &bead.Bead{ID: "parent-001", Title: "Parent"},
		Result:        &runtypes.IterationResult{Output: "build output"},
		Model:         "sonnet",
		Tier:          provider.TierMedium,
		BuildProvider: "test-provider",
		PromptCtx:     &prompt.Context{WorkDir: "/tmp/test-work"},
		StartCommit:   "start-commit-abc",
		Iteration:     3,
		RunDeadline:   time.Time{}, // no deadline
	}
}

// --- RunPostSuccess tests ---

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_RunsLightReviewAndLogs(t *testing.T) {
	// RunPostSuccess should call RunLight, then ApplyResult, then WriteReviewLog
	// when the review returns a result.
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Looks good","fixes_applied":[],"beads_to_create":[{"title":"Minor fix","description":"Details","priority":2,"labels":["enhancement"]}],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test-provider"
		},
	}

	beadClient := &mockBeadClient{}
	mockLog := &mockIterationLogger{}

	rev := NewReviewer(cfg, router, beadClient, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff --git a/foo.go b/foo.go\n+line", nil
	}, mockLog)

	bc := newTestBeadContext()

	// Expected failure: RunPostSuccess does not exist on Reviewer
	err := rev.RunPostSuccess(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunPostSuccess returned error: %v", err)
	}

	// Verify bead was created from review findings
	if len(beadClient.created) != 1 {
		t.Fatalf("expected 1 bead created, got %d", len(beadClient.created))
	}
	if beadClient.created[0].title != "Minor fix" {
		t.Errorf("created bead title = %q, want %q", beadClient.created[0].title, "Minor fix")
	}

	// Verify review was logged
	if len(mockLog.reviews) != 1 {
		t.Fatalf("expected 1 review log, got %d", len(mockLog.reviews))
	}
	if mockLog.reviews[0].ReviewType != "light" {
		t.Errorf("review log type = %q, want %q", mockLog.reviews[0].ReviewType, "light")
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_ReviewFailureIsNonBlocking(t *testing.T) {
	// When the light review itself fails (provider error), RunPostSuccess
	// should log a warning and return nil (not propagate the error).
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return nil, fmt.Errorf("provider timeout")
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	rev := NewReviewer(cfg, router, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, nil)

	bc := newTestBeadContext()

	// Expected failure: RunPostSuccess does not exist
	err := rev.RunPostSuccess(context.Background(), bc)
	if err != nil {
		t.Errorf("RunPostSuccess should return nil on review failure, got: %v", err)
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_NilReviewResultIsNoOp(t *testing.T) {
	// When RunLight returns nil result (e.g., no diff or deadline expired),
	// RunPostSuccess should return nil without logging or creating beads.
	cfg := newTestConfig()

	// gitDiffFn returns empty diff → RunLight returns nil
	rev := NewReviewer(cfg, &mockRouter{}, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "", nil // no diff
	}, &mockIterationLogger{})

	bc := newTestBeadContext()

	// Expected failure: RunPostSuccess does not exist
	err := rev.RunPostSuccess(context.Background(), bc)
	if err != nil {
		t.Errorf("RunPostSuccess should return nil when review skipped, got: %v", err)
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_RevalidatesAfterFixesApplied(t *testing.T) {
	// When the light review applies fixes, RunPostSuccess should re-validate.
	// If validation passes, no error should be returned.
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Fixed formatting","fixes_applied":["gofmt"],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	validationCalled := false
	validateFn := func(ctx context.Context, commands []string, workDir string) (bool, error) {
		validationCalled = true
		if workDir != "/tmp/test-work" {
			t.Errorf("validation workDir = %q, want %q", workDir, "/tmp/test-work")
		}
		return true, nil // passes
	}

	rev := NewReviewer(cfg, router, &mockBeadClient{}, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, &mockIterationLogger{})
	rev.SetValidateFn(validateFn)

	bc := newTestBeadContext()

	err := rev.RunPostSuccess(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunPostSuccess returned unexpected error: %v", err)
	}

	if !validationCalled {
		t.Error("RunPostSuccess should call validation when fixes were applied")
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_SetsReviewBrokeValidation(t *testing.T) {
	// When the light review applies fixes that break validation,
	// RunPostSuccess should set bc.Result.ReviewBrokeValidation = true
	// and return an error.
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Applied fix","fixes_applied":["bad fix"],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	// Validation will fail after fixes
	validateFn := func(ctx context.Context, commands []string, workDir string) (bool, error) {
		return false, nil // validation fails
	}

	rev := NewReviewer(cfg, router, &mockBeadClient{}, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, &mockIterationLogger{})
	rev.SetValidateFn(validateFn)

	bc := newTestBeadContext()

	err := rev.RunPostSuccess(context.Background(), bc)

	// err should be non-nil (review fixes broke validation)
	if err == nil {
		t.Error("RunPostSuccess should return error when review fixes break validation")
	}
	// bc.Result.ReviewBrokeValidation should be true
	if !bc.Result.ReviewBrokeValidation {
		t.Error("bc.Result.ReviewBrokeValidation should be true when review fixes break validation")
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_AppendsValidationOutputOnFailure(t *testing.T) {
	// When review fixes break validation, RunPostSuccess should append
	// the validation failure output to bc.Result.Output.
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"Fixed","fixes_applied":["fix"],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	rev := NewReviewer(cfg, router, &mockBeadClient{}, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, &mockIterationLogger{})
	rev.SetValidateFn(func(ctx context.Context, commands []string, workDir string) (bool, error) {
		return false, nil // validation fails
	})

	bc := newTestBeadContext()
	originalOutput := bc.Result.Output

	_ = rev.RunPostSuccess(context.Background(), bc)

	// The output should have been appended to with validation failure info
	if bc.Result.Output == originalOutput {
		t.Error("bc.Result.Output should be modified when review re-validation fails")
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_SkipsValidationWhenNoFixes(t *testing.T) {
	// When the review returns no fixes_applied, RunPostSuccess should not
	// run re-validation — just apply results and log.
	cfg := newTestConfig()

	prov := &mockProvider{
		name: "test",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"LGTM","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "test"
		},
	}

	validationCalled := false
	validateFn := func(ctx context.Context, commands []string, workDir string) (bool, error) {
		validationCalled = true
		return true, nil
	}

	rev := NewReviewer(cfg, router, &mockBeadClient{}, &mockPromptRenderer{}, func(string) (string, error) {
		return "diff content", nil
	}, &mockIterationLogger{})
	rev.SetValidateFn(validateFn)

	bc := newTestBeadContext()

	err := rev.RunPostSuccess(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunPostSuccess returned unexpected error: %v", err)
	}

	if validationCalled {
		t.Error("RunPostSuccess should not call validation when no fixes were applied")
	}
}

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_UsesBeadContextFieldsCorrectly(t *testing.T) {
	// RunPostSuccess should pass the correct fields from BeadContext
	// to RunLight: bead, parent, startCommit, model, iteration, deadline, buildProvider.
	cfg := newTestConfig()

	var capturedArgs struct {
		startCommit   string
		buildModel    string
		buildProvider string
	}

	prov := &mockProvider{
		name: "captured-provider",
		runFn: func(ctx context.Context, p string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `{"passed":true,"summary":"OK","fixes_applied":[],"beads_to_create":[],"backlog_items":[]}`,
			}, nil
		},
	}

	router := &mockRouter{
		selectFn: func(phase, tier string) (provider.Provider, string) {
			return prov, "captured-provider"
		},
	}

	gitDiffFn := func(startCommit string) (string, error) {
		capturedArgs.startCommit = startCommit
		return "diff content", nil
	}

	rev := NewReviewer(cfg, router, &mockBeadClient{}, &mockPromptRenderer{
		renderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			capturedArgs.buildModel = ctx.Model
			return "review prompt", nil
		},
	}, gitDiffFn, &mockIterationLogger{})

	bc := newTestBeadContext()
	bc.StartCommit = "my-start-commit"
	bc.Model = "opus"
	bc.BuildProvider = "my-build-provider"

	// Expected failure: RunPostSuccess does not exist
	_ = rev.RunPostSuccess(context.Background(), bc)

	if capturedArgs.startCommit != "my-start-commit" {
		t.Errorf("git diff startCommit = %q, want %q", capturedArgs.startCommit, "my-start-commit")
	}
}

// --- RunPostSuccess: Signature acceptance ---

// Expected failure: Reviewer.RunPostSuccess method does not exist yet
func TestRunPostSuccess_AcceptsBeadContext(t *testing.T) {
	// RunPostSuccess must accept a *runtypes.BeadContext and return error.
	// This tests the method signature through the public API.
	cfg := newTestConfig()

	rev := NewReviewer(cfg, &mockRouter{}, nil, &mockPromptRenderer{}, func(string) (string, error) {
		return "", nil
	}, nil)

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "sig-test", Priority: 1},
		Result:    &runtypes.IterationResult{},
		Model:     "haiku",
		PromptCtx: &prompt.Context{WorkDir: "/tmp"},
	}

	// Expected failure: RunPostSuccess does not exist
	var err error
	err = rev.RunPostSuccess(context.Background(), bc)

	// The method must compile and return an error type
	_ = err
}
