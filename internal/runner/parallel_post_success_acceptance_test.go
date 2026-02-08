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
// "When both learn_from_success and review.enabled are true, extractSuccessLearning
// and runLightReview execute concurrently after validation passes."
//
// This test verifies concurrent execution by measuring wall-clock time. If the stages
// run sequentially, total time will be ~500ms (200ms + 300ms). If concurrent, total
// time will be ~300ms (max of 200ms and 300ms).
func TestAcceptance_ParallelPostSuccess_ConcurrentExecution(t *testing.T) {
	const learningDuration = 200 * time.Millisecond
	const reviewDuration = 300 * time.Millisecond

	var learningStartTime, reviewStartTime time.Time
	var mu sync.Mutex
	learningCalled := false
	reviewCalled := false

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build complete"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Detect which stage is calling based on prompt content
			isLearning := strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning")
			isReview := strings.Contains(prompt, "review") || strings.Contains(prompt, "Review")

			mu.Lock()
			if isLearning {
				learningStartTime = time.Now()
				learningCalled = true
			} else if isReview {
				reviewStartTime = time.Now()
				reviewCalled = true
			}
			mu.Unlock()

			if isLearning {
				time.Sleep(learningDuration)
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			} else if isReview {
				time.Sleep(reviewDuration)
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
			Models: config.ModelsConfig{Validation: "haiku"},
			Review: config.ReviewConfig{Enabled: true, Model: "sonnet"},
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

	start := time.Now()
	err = r.Run(context.Background(), 1, time.Time{}, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify both stages were called
	mu.Lock()
	if !learningCalled {
		t.Fatal("learning extraction was not called")
	}
	if !reviewCalled {
		t.Fatal("review was not called")
	}

	// Verify concurrent execution by checking start times
	timeDiff := reviewStartTime.Sub(learningStartTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	mu.Unlock()

	// If stages run concurrently, they should start within ~50ms of each other
	// If sequential, the second will start after the first completes (~200ms or ~300ms later)
	if timeDiff > 100*time.Millisecond {
		t.Errorf("stages did not run concurrently - start time difference: %v (expected <100ms)", timeDiff)
	}

	// Verify total execution time is closer to max(200ms, 300ms) = 300ms than sum(200ms, 300ms) = 500ms
	// Allow 150ms overhead for test infrastructure
	maxExpected := reviewDuration + 150*time.Millisecond // ~450ms
	sumExpected := learningDuration + reviewDuration     // 500ms

	if elapsed > sumExpected {
		t.Errorf("execution took too long (%v), suggesting sequential execution (expected <%v for concurrent)",
			elapsed, maxExpected)
	}

	// Should take at least the duration of the longer stage
	if elapsed < reviewDuration {
		t.Errorf("execution was too fast (%v), expected at least %v", elapsed, reviewDuration)
	}
}

// TestAcceptance_ParallelPostSuccess_LearningFailureDoesNotBlockReview verifies acceptance criterion:
// "A failure in learning extraction does not prevent or delay the review, and vice versa."
func TestAcceptance_ParallelPostSuccess_LearningFailureDoesNotBlockReview(t *testing.T) {
	reviewCompleted := false
	learningFailed := false
	var mu sync.Mutex

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
				mu.Lock()
				learningFailed = true
				mu.Unlock()
				return nil, fmt.Errorf("learning extraction failed")
			}

			// Review succeeds
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			reviewCompleted = true
			mu.Unlock()
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
			Models: config.ModelsConfig{Validation: "haiku"},
			Review: config.ReviewConfig{Enabled: true, Model: "sonnet"},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() should succeed even if learning fails: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !learningFailed {
		t.Fatal("learning should have been called and failed")
	}
	if !reviewCompleted {
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
// This test verifies that when only learning is enabled, review is not called.
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
			Models: config.ModelsConfig{Validation: "haiku"},
			Review: config.ReviewConfig{Enabled: false}, // Review disabled
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
// This test verifies that when only review is enabled, learning is not called.
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
			Models: config.ModelsConfig{Validation: "haiku"},
			Review: config.ReviewConfig{Enabled: true, Model: "sonnet"},
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

	if learningCalled {
		t.Error("expected learning NOT to be called when disabled")
	}
	if !reviewCalled {
		t.Error("expected review to be called when enabled")
	}
}

// TestAcceptance_ParallelPostSuccess_ReviewRevalidationErrorPropagates verifies acceptance criterion:
// "Review re-validation (when fixes are applied) still works correctly within its goroutine
// and its error still propagates to the caller."
//
// This test verifies that when review applies fixes and re-validation fails, the error
// propagates to the caller and the bead is NOT closed, even when both stages run concurrently.
func TestAcceptance_ParallelPostSuccess_ReviewRevalidationErrorPropagates(t *testing.T) {
	learningCompleted := false
	reviewCompleted := false
	revalidationAttempted := false
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
			mu.Lock()
			revalidationAttempted = true
			mu.Unlock()
			return &claude.Result{Success: false, Output: "VALIDATION_FAILED: review broke something"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			isLearning := strings.Contains(prompt, "extract") || strings.Contains(prompt, "learning")
			isReview := strings.Contains(prompt, "review") || strings.Contains(prompt, "Review")

			if isLearning {
				time.Sleep(100 * time.Millisecond) // Simulate learning work
				mu.Lock()
				learningCompleted = true
				mu.Unlock()
				return &claude.Result{Success: true, Output: `{"learning":"test","category":"patterns"}`}, nil
			}

			if isReview {
				time.Sleep(150 * time.Millisecond) // Simulate review work
				mu.Lock()
				reviewCompleted = true
				mu.Unlock()
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
			Models: config.ModelsConfig{Validation: "haiku"},
			Review: config.ReviewConfig{Enabled: true, Model: "sonnet"},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	err = r.Run(context.Background(), 1, time.Time{}, false)

	// Verify that the error propagated (Run should fail)
	if err == nil {
		t.Fatal("Expected Run() to fail when review re-validation fails, but it succeeded")
	}
	if !strings.Contains(err.Error(), "review fixes broke validation") {
		t.Errorf("Expected error to mention 'review fixes broke validation', got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify both stages attempted to complete
	if !learningCompleted {
		t.Error("learning should have completed")
	}
	if !reviewCompleted {
		t.Error("review should have completed")
	}
	if !revalidationAttempted {
		t.Error("re-validation should have been attempted")
	}

	// Verify bead was NOT closed (due to re-validation failure)
	if len(beads.ClosedIDs) != 0 {
		t.Errorf("expected bead NOT to be closed when re-validation fails, but got closed: %v", beads.ClosedIDs)
	}
}
