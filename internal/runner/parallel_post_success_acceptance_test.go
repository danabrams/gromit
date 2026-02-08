//go:build acceptance

package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestAcceptance_ParallelPostSuccess_ConcurrentExecution verifies acceptance criterion:
// "Replace timing-based concurrency assertions with channel/synchronization-based
// assertions that prove concurrency without wall-clock thresholds."
//
// Uses a barrier pattern: both stages must arrive at the barrier before either can
// proceed. If execution were sequential, this would deadlock (caught by test timeout).
// No wall-clock timing thresholds are used.
func TestAcceptance_ParallelPostSuccess_ConcurrentExecution(t *testing.T) {
	// Barrier channels: each stage signals arrival, then waits for the other.
	// If stages run sequentially, the first stage blocks forever waiting for
	// the second to arrive → test timeout → proof of sequential execution.
	learningArrived := make(chan struct{})
	reviewArrived := make(chan struct{})

	// Track that both stages actually ran (buffered to avoid blocking)
	learningStarted := make(chan struct{}, 1)
	reviewStarted := make(chan struct{}, 1)

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			isLearning := strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning")
			isReview := strings.Contains(prompt, "review") || strings.Contains(prompt, "Review")

			if isLearning {
				// Signal that learning has started
				select {
				case learningStarted <- struct{}{}:
				default:
				}
				// Signal arrival at barrier
				close(learningArrived)
				// Wait for review to also arrive (proves concurrency)
				<-reviewArrived
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			}
			if isReview {
				// Signal that review has started
				select {
				case reviewStarted <- struct{}{}:
				default:
				}
				// Signal arrival at barrier
				close(reviewArrived)
				// Wait for learning to also arrive (proves concurrency)
				<-learningArrived
				return &claude.Result{Success: true, Output: `{"summary":"ok","findings":[],"fixes_applied":[]}`}, nil
			}
			return &claude.Result{Success: true, Output: "unknown call"}, nil
		},
	}

	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	lf.SetFilter(func(content string) (bool, error) { return true, nil })

	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review the code", nil
		},
	}

	learnFromSuccessEnabled := true
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:       "test-concurrent",
				Title:    "Test concurrent execution",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessEnabled,
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"echo VALIDATION_PASSED"},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: true, Model: "sonnet"},
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{
			Beads:    beads,
			Claude:   mockClaude,
			Analyzer: &mockFailureAnalyzer{},
			Renderer: mockRend,
			Logger:   &mockIterationLogger{},
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}

	// Run with a generous timeout — the barrier pattern deadlocks if sequential,
	// so the test will fail on timeout rather than a timing assertion.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = r.Run(ctx, 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify both stages actually started
	select {
	case <-learningStarted:
		// good
	default:
		t.Fatal("learning extraction was never started")
	}

	select {
	case <-reviewStarted:
		// good
	default:
		t.Fatal("review was never started")
	}

	// The barrier pattern proves concurrency: if both stages completed without
	// deadlocking, they must have been running concurrently (each waited for the
	// other at the barrier).
}

// TestAcceptance_ParallelPostSuccess_LearningFailureDoesNotBlockReview verifies acceptance criterion:
// "A failure in learning extraction does not prevent or delay the review, and vice versa."
func TestAcceptance_ParallelPostSuccess_LearningFailureDoesNotBlockReview(t *testing.T) {
	reviewCompleted := make(chan struct{}, 1)
	learningFailed := make(chan struct{}, 1)

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			isLearning := strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning")

			if isLearning {
				select {
				case learningFailed <- struct{}{}:
				default:
				}
				return nil, fmt.Errorf("learning extraction failed")
			}

			// Review succeeds
			select {
			case reviewCompleted <- struct{}{}:
			default:
			}
			return &claude.Result{Success: true, Output: `{"summary":"ok","findings":[],"fixes_applied":[]}`}, nil
		},
	}

	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	lf.SetFilter(func(content string) (bool, error) { return true, nil })

	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review the code", nil
		},
	}

	learnFromSuccessEnabled := true
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{ID: "test-failure", Title: "Test", Priority: 1, Labels: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessEnabled,
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"echo VALIDATION_PASSED"},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: true, Model: "sonnet"},
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() should succeed even if learning fails: %v", err)
	}

	// Verify learning was called and failed
	select {
	case <-learningFailed:
		// good
	default:
		t.Fatal("learning should have been called and failed")
	}

	// Verify review completed despite learning failure
	select {
	case <-reviewCompleted:
		// good
	default:
		t.Fatal("review should have completed despite learning failure")
	}

	// Verify bead was closed successfully
	if len(beads.ClosedIDs) != 1 {
		t.Errorf("expected bead to be closed despite learning failure, got: %v", beads.ClosedIDs)
	}
}

// TestAcceptance_ParallelPostSuccess_OnlyLearningEnabled verifies acceptance criterion:
// "When only one stage is enabled, it runs without goroutine overhead."
//
// When only learning is enabled, review must not be called.
func TestAcceptance_ParallelPostSuccess_OnlyLearningEnabled(t *testing.T) {
	learningCalled := false
	reviewCalled := false
	var mu sync.Mutex

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			mu.Lock()
			defer mu.Unlock()

			if strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning") {
				learningCalled = true
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			}
			if strings.Contains(prompt, "review") || strings.Contains(prompt, "Review") {
				reviewCalled = true
				return &claude.Result{Success: true, Output: `{"summary":"ok","findings":[],"fixes_applied":[]}`}, nil
			}
			return &claude.Result{Success: true, Output: "unknown"}, nil
		},
	}

	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	lf.SetFilter(func(content string) (bool, error) { return true, nil })

	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
	}

	learnFromSuccessEnabled := true
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{ID: "test-only-learning", Title: "Test", Priority: 1, Labels: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessEnabled,
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"echo VALIDATION_PASSED"},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: false}, // Review disabled
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !learningCalled {
		t.Error("expected learning to be called when enabled")
	}
	if reviewCalled {
		t.Error("expected review NOT to be called when disabled")
	}
}

// TestAcceptance_ParallelPostSuccess_OnlyReviewEnabled verifies acceptance criterion:
// "When only one stage is enabled, it runs without goroutine overhead."
//
// When only review is enabled, learning must not be called.
func TestAcceptance_ParallelPostSuccess_OnlyReviewEnabled(t *testing.T) {
	learningCalled := false
	reviewCalled := false
	var mu sync.Mutex

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			mu.Lock()
			defer mu.Unlock()

			if strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning") {
				learningCalled = true
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			}
			if strings.Contains(prompt, "review") || strings.Contains(prompt, "Review") {
				reviewCalled = true
				return &claude.Result{Success: true, Output: `{"summary":"ok","findings":[],"fixes_applied":[]}`}, nil
			}
			return &claude.Result{Success: true, Output: "unknown"}, nil
		},
	}

	mockRend := &mockPromptRenderer{
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review the code", nil
		},
	}

	learnFromSuccessDisabled := false
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{ID: "test-only-review", Title: "Test", Priority: 1, Labels: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessDisabled, // Learning disabled
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"echo VALIDATION_PASSED"},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: true, Model: "sonnet"},
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if learningCalled {
		t.Error("expected learning NOT to be called when disabled")
	}
	if !reviewCalled {
		t.Error("expected review to be called when enabled")
	}
}

// TestAcceptance_ParallelPostSuccess_NeitherEnabled verifies acceptance criterion:
// "When both stages are disabled, neither runs."
//
// This test catches the bug where RunFn is called even when review is disabled,
// because the runner code incorrectly invokes review despite review.enabled=false.
func TestAcceptance_ParallelPostSuccess_NeitherEnabled(t *testing.T) {
	runFnCalled := false
	var mu sync.Mutex

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			mu.Lock()
			runFnCalled = true
			mu.Unlock()
			return &claude.Result{Success: true, Output: "should not be called"}, nil
		},
	}

	learnFromSuccessDisabled := false
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{ID: "test-neither", Title: "Test", Priority: 1, Labels: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessDisabled, // Learning disabled
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"echo VALIDATION_PASSED"},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: false}, // Review disabled
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockRenderer{}, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// When both learning and review are disabled, Claude's Run() should not be
	// called at all for post-success stages. The only Claude calls should be
	// StreamRun (build) and RunValidation (validation).
	if runFnCalled {
		t.Error("expected Run() NOT to be called when both learning and review are disabled")
	}

	// Bead should still be closed successfully
	if len(beads.ClosedIDs) != 1 {
		t.Errorf("expected bead to be closed, got: %v", beads.ClosedIDs)
	}
}

// TestAcceptance_ParallelPostSuccess_ReviewRevalidationErrorPropagates verifies acceptance criterion:
// "Review re-validation (when fixes are applied) still works correctly within its goroutine
// and its error propagates to the caller."
//
// When review applies fixes and re-validation fails, Run() must return an error and the
// bead must NOT be closed. This catches the bug where runPostSuccessParallel swallows
// the re-validation error.
func TestAcceptance_ParallelPostSuccess_ReviewRevalidationErrorPropagates(t *testing.T) {
	learningCompleted := make(chan struct{}, 1)
	reviewCompleted := make(chan struct{}, 1)
	revalidationAttempted := make(chan struct{}, 1)
	validationCallCount := 0
	var mu sync.Mutex

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			mu.Lock()
			validationCallCount++
			callNum := validationCallCount
			mu.Unlock()

			// First call is the initial validation (passes)
			if callNum == 1 {
				return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
			}

			// Second call is re-validation after review fixes (fails)
			select {
			case revalidationAttempted <- struct{}{}:
			default:
			}
			return &claude.Result{Success: false, Output: "VALIDATION_FAILED: review broke something"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			isLearning := strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning")
			isReview := strings.Contains(prompt, "review") || strings.Contains(prompt, "Review")

			if isLearning {
				select {
				case learningCompleted <- struct{}{}:
				default:
				}
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			}

			if isReview {
				select {
				case reviewCompleted <- struct{}{}:
				default:
				}
				// Review returns fixes applied, which triggers re-validation
				return &claude.Result{Success: true, Output: `{"summary":"Fixed 2 issues","findings":[],"fixes_applied":["fix1.go","fix2.go"]}`}, nil
			}

			return &claude.Result{Success: true, Output: "unknown"}, nil
		},
	}

	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	lf.SetFilter(func(content string) (bool, error) { return true, nil })

	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review the code", nil
		},
	}

	learnFromSuccessEnabled := true
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:       "test-revalidation",
				Title:    "Test review re-validation error propagation",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessEnabled,
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: true, Model: "sonnet"},
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)

	// CRITICAL: Run() must fail when review re-validation fails.
	// This catches the bug where runPostSuccessParallel swallows the error.
	if err == nil {
		t.Fatal("Expected Run() to fail when review re-validation fails, but it succeeded")
	}
	if !strings.Contains(err.Error(), "review fixes broke validation") {
		t.Errorf("Expected error to mention 'review fixes broke validation', got: %v", err)
	}

	// Verify both stages ran
	select {
	case <-learningCompleted:
		// good
	default:
		t.Error("learning should have completed")
	}

	select {
	case <-reviewCompleted:
		// good
	default:
		t.Error("review should have completed")
	}

	// Verify re-validation was attempted
	select {
	case <-revalidationAttempted:
		// good
	default:
		t.Error("re-validation should have been attempted")
	}

	// Verify bead was NOT closed (due to re-validation failure)
	if len(beads.ClosedIDs) != 0 {
		t.Errorf("expected bead NOT to be closed when re-validation fails, but got closed: %v", beads.ClosedIDs)
	}
}

// TestAcceptance_ParallelPostSuccess_ReviewStartsWhenBothEnabled verifies acceptance criterion:
// "All parallel post-success tests pass reliably."
//
// This catches the bug where review never starts because runLightReview fails
// when gitDiffFn is not set or git operations fail in test environments.
// Both stages must execute their Claude RunFn calls when both are enabled.
func TestAcceptance_ParallelPostSuccess_ReviewStartsWhenBothEnabled(t *testing.T) {
	learningRan := make(chan struct{}, 1)
	reviewRan := make(chan struct{}, 1)

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			isLearning := strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning")
			isReview := strings.Contains(prompt, "review") || strings.Contains(prompt, "Review")

			if isLearning {
				select {
				case learningRan <- struct{}{}:
				default:
				}
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			}
			if isReview {
				select {
				case reviewRan <- struct{}{}:
				default:
				}
				return &claude.Result{Success: true, Output: `{"summary":"Looks good","findings":[],"fixes_applied":[]}`}, nil
			}
			return &claude.Result{Success: true, Output: "unknown"}, nil
		},
	}

	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	lf.SetFilter(func(content string) (bool, error) { return true, nil })

	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review the code", nil
		},
	}

	learnFromSuccessEnabled := true
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:       "test-both-start",
				Title:    "Test both stages start",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessEnabled,
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"echo VALIDATION_PASSED"},
			},
			Models:   config.ModelsConfig{Validation: "haiku"},
			Review:   config.ReviewConfig{Enabled: true, Model: "sonnet"},
			Precheck: config.PrecheckConfig{Enabled: boolPtr(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Both stages MUST have called their RunFn
	select {
	case <-learningRan:
		// good
	default:
		t.Error("learning extraction never called Claude RunFn")
	}

	select {
	case <-reviewRan:
		// good
	default:
		t.Error("review never called Claude RunFn — review stage failed to start or execute")
	}

	// Bead should be closed
	if len(beads.ClosedIDs) != 1 {
		t.Errorf("expected bead to be closed, got: %v", beads.ClosedIDs)
	}
}
