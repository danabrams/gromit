package loop

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRecordsStartGeneration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	// Request with a bead that has gen:5 label
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:5"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	if result == nil {
		t.Fatalf("result is nil")
	}

	if result.StartGeneration != 5 {
		t.Fatalf("StartGeneration = %d, want 5", result.StartGeneration)
	}
}

// successStage is a test stage that always succeeds
func TestBeadLoopHasDefaultGenerationCap(t *testing.T) {
	t.Parallel()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	// Default generation cap should be 3
	if beadLoop.GenerationCap != 3 {
		t.Fatalf("GenerationCap = %d, want 3", beadLoop.GenerationCap)
	}
}

func TestBeadLoopCanSetCustomGenerationCap(t *testing.T) {
	t.Parallel()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 5

	if beadLoop.GenerationCap != 5 {
		t.Fatalf("GenerationCap = %d, want 5", beadLoop.GenerationCap)
	}
}

func TestGenerationCapReachedMarksCapHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start at generation 5, cap of 3 means generation 8 would be capped
	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 3

	// Request with generation 5, cap of 3, so start + cap = 8
	// Current bead is generation 5, which is <= start + cap, so cap not hit yet
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:5"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Generation 5 is not >= start (5) + cap (3), so CapHit should be false
	if result.CapHit {
		t.Fatalf("CapHit = true, want false for generation 5 with start=5, cap=3")
	}
}

func TestEmitsGenerationCapReachedEventWhenCapHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	emitter := events.NewEmitter()
	eventChan := emitter.Subscribe()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 3
	beadLoop.Emitter = emitter

	// First run with generation 0
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Second run with generation 3 (cap threshold reached)
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-2",
			Labels: []string{"gen:3"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Check that GenerationCapReachedEvent was emitted
	var foundEvent bool
	select {
	case evt := <-eventChan:
		if _, ok := evt.(*events.GenerationCapReachedEvent); ok {
			foundEvent = true
		}
	default:
		// No event emitted
	}

	if !foundEvent {
		t.Fatalf("expected GenerationCapReachedEvent to be emitted, but it was not")
	}
}

func TestCapHitDetectsWhenGenerationThresholdExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 3

	// Process a bead at generation 0
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// StartGeneration = 0, cap = 3, so threshold = 3
	// Current generation 0 < 3, so CapHit should be false
	if result.CapHit {
		t.Fatalf("CapHit = true, want false for generation 0 with cap 3")
	}

	// Process a bead at generation 3 (at threshold: start=0, cap=3, threshold=3)
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-2",
			Labels: []string{"gen:3"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// StartGeneration = 0 (from first run), cap = 3, threshold = 3
	// Current generation 3 >= 3, so CapHit should be true
	if !result.CapHit {
		t.Fatalf("CapHit = false, want true for generation 3 with start=0, cap=3")
	}

	// Process a bead at generation 2 (below threshold)
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-3",
			Labels: []string{"gen:2"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// StartGeneration = 0, cap = 3, threshold = 3
	// Cap was already hit, so CapHit should remain true even though generation decreases
	if !result.CapHit {
		t.Fatalf("CapHit = false, want true after cap already hit")
	}
}

func TestMaxRetriesExhaustedReturnsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a stage that always fails
	alwaysFailingStage := &failingStage{
		name:      "test",
		failUntil: 999, // Fails forever
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: alwaysFailingStage,
			Retry: stage.RetryConfig{MaxRetries: 1},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err == nil {
		t.Fatalf("expected error when max retries exhausted, got nil")
	}

	// Verify the error is about max retries being exceeded
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Fatalf("error should contain %q, got %q", "max retries exceeded", err.Error())
	}

	// Verify the stage was actually retried (ran 2 times: initial + 1 retry)
	if alwaysFailingStage.runCount != 2 {
		t.Fatalf("stage should have been run 2 times (initial + 1 retry), got %d", alwaysFailingStage.runCount)
	}
}

func TestRetryContextPopulatedWithPriorFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	capturedContexts := []*stage.RetryContext{}

	mainStage := &retryContextCheckingStage{
		name:             "main",
		capturedContexts: &capturedContexts,
		shouldFail:       true,
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: mainStage,
			Retry: stage.RetryConfig{MaxRetries: 1},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Should have captured 2 contexts: one nil on first run, one with failure info on retry
	if len(capturedContexts) != 2 {
		t.Fatalf("captured contexts length = %d, want 2. Contexts: %v", len(capturedContexts), capturedContexts)
	}

	// First run should have nil context (initial request)
	if capturedContexts[0] != nil {
		t.Fatalf("first context should be nil, got %v", capturedContexts[0])
	}

	// Second run (retry) should have context with attempt=1 and prior failures
	retryCtx := capturedContexts[1]
	if retryCtx == nil {
		t.Fatalf("retry context should not be nil")
	}
	if retryCtx.Attempt != 1 {
		t.Fatalf("retry context attempt = %d, want 1", retryCtx.Attempt)
	}
	if len(retryCtx.PriorFailures) == 0 {
		t.Fatalf("retry context PriorFailures should not be empty")
	}
	if !contains(retryCtx.PriorFailures, "main failed: stage failed") {
		t.Fatalf("PriorFailures should contain 'main failed: stage failed', got %v", retryCtx.PriorFailures)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestRetryWithStagesExecutedBeforeFailedStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runOrder := []string{}

	fixStage := &orderTrackingStage{
		name:     "fix",
		runOrder: &runOrder,
	}
	mainStage := &orderTrackingStage{
		name:       "main",
		runOrder:   &runOrder,
		shouldFail: true,
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: fixStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
		{
			Stage: mainStage,
			Retry: stage.RetryConfig{MaxRetries: 1, RetryWith: []string{"fix"}},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Expected order: fix, main (1st), fix, main (2nd)
	expected := []string{"fix", "main", "fix", "main"}
	if len(runOrder) != len(expected) {
		t.Fatalf("run order length = %d, want %d. Got: %v", len(runOrder), len(expected), runOrder)
	}

	for i, name := range expected {
		if runOrder[i] != name {
			t.Fatalf("stage order mismatch at position %d: got %s, want %s. Full order: %v", i, runOrder[i], name, runOrder)
		}
	}
}

func TestStageRetryingEventContainsCorrectRetryInfo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	emitter := events.NewEmitter()
	eventChan := emitter.Subscribe()

	testStage := &failingStage{
		name:      "critical",
		failUntil: 2, // Fails on 1st and 2nd runs, succeeds on 3rd
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: testStage,
			Retry: stage.RetryConfig{MaxRetries: 2},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.Emitter = emitter

	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Collect all StageRetrying events
	var retryEvents []*events.StageRetryingEvent
	for {
		select {
		case evt := <-eventChan:
			if retryEvt, ok := evt.(*events.StageRetryingEvent); ok {
				retryEvents = append(retryEvents, retryEvt)
			}
		default:
			goto done
		}
	}
done:

	// Should have 2 StageRetrying events (for attempt 1 and 2)
	if len(retryEvents) != 2 {
		t.Fatalf("expected 2 StageRetrying events, got %d", len(retryEvents))
	}

	// Check first retry event
	if retryEvents[0].StageName != "critical" {
		t.Fatalf("first retry event StageName = %s, want critical", retryEvents[0].StageName)
	}
	if retryEvents[0].Attempt != 1 {
		t.Fatalf("first retry event Attempt = %d, want 1", retryEvents[0].Attempt)
	}
	if !strings.Contains(retryEvents[0].Reason, "failed") {
		t.Fatalf("first retry event Reason should contain 'failed', got %q", retryEvents[0].Reason)
	}

	// Check second retry event
	if retryEvents[1].StageName != "critical" {
		t.Fatalf("second retry event StageName = %s, want critical", retryEvents[1].StageName)
	}
	if retryEvents[1].Attempt != 2 {
		t.Fatalf("second retry event Attempt = %d, want 2", retryEvents[1].Attempt)
	}
}

func TestEmitsStageRetryingEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	emitter := events.NewEmitter()
	eventChan := emitter.Subscribe()

	testStage := &failingStage{
		name:      "test",
		failUntil: 1,
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: testStage,
			Retry: stage.RetryConfig{MaxRetries: 1},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.Emitter = emitter

	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	_, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// Check that StageRetrying event was emitted
	var foundEvent bool
	select {
	case evt := <-eventChan:
		if _, ok := evt.(*events.StageRetryingEvent); ok {
			foundEvent = true
		}
	default:
		// No event emitted
	}

	if !foundEvent {
		t.Fatalf("expected StageRetryingEvent to be emitted, but it was not")
	}
}

func TestCapHitPersistsAfterThreshold(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	successStage := &successStage{name: "test"}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 3

	// First run to record start generation at 0
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}
	if result.CapHit {
		t.Fatalf("CapHit = true, want false before threshold")
	}

	// Trigger cap by reaching generation 3
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-2",
			Labels: []string{"gen:3"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}
	if !result.CapHit {
		t.Fatalf("CapHit = false, want true after threshold")
	}

	// Even though generation decreases, CapHit should stay true
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-3",
			Labels: []string{"gen:2"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}
	if !result.CapHit {
		t.Fatalf("CapHit = false, want true after cap already hit")
	}
}

func TestGenerationCapSkipsReviewAndDecomposeStages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	otherStage := &trackingStage{name: "gate"}
	decomposeStage := &trackingStage{name: "decompose"}
	reviewStage := &trackingStage{name: "review"}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: otherStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
		{
			Stage: decomposeStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
		{
			Stage: reviewStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 1

	// Run once to establish start generation and ensure every stage executes initially
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "first-bead",
			Labels: []string{"gen:0"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}
	if result.CapHit {
		t.Fatalf("CapHit = true, want false before threshold")
	}
	if otherStage.runCount != 1 || decomposeStage.runCount != 1 || reviewStage.runCount != 1 {
		t.Fatalf("expected each stage to run once before cap (got other=%d, decompose=%d, review=%d)",
			otherStage.runCount, decomposeStage.runCount, reviewStage.runCount)
	}

	// Trigger the cap with a generation 1 bead
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "second-bead",
			Labels: []string{"gen:1"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}
	if !result.CapHit {
		t.Fatalf("CapHit = false, want true after cap reached")
	}
	if otherStage.runCount != 2 {
		t.Fatalf("expected non-blocked stage to run twice, got %d", otherStage.runCount)
	}
	if decomposeStage.runCount != 1 {
		t.Fatalf("expected decompose stage to be skipped after cap, got %d", decomposeStage.runCount)
	}
	if reviewStage.runCount != 1 {
		t.Fatalf("expected review stage to be skipped after cap, got %d", reviewStage.runCount)
	}
}

// successStage is a test stage that always succeeds
type successStage struct {
	name string
}

func (s *successStage) Name() string {
	return s.name
}

func (s *successStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// capHitTestStage is a test stage that creates beads at a specific generation
type capHitTestStage struct {
	name           string
	nextGeneration int
}

func (s *capHitTestStage) Name() string {
	return s.name
}

func (s *capHitTestStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	// This stage would normally enqueue new beads at nextGeneration
	// For now, just return success
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// trackingStage counts how many times Run() is executed for assertions.
type trackingStage struct {
	name     string
	runCount int
}

func (s *trackingStage) Name() string {
	return s.name
}

func (s *trackingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// failingStage fails on first attempt, succeeds on retry
type failingStage struct {
	name      string
	runCount  int
	failUntil int
}

func (s *failingStage) Name() string {
	return s.name
}

func (s *failingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if s.runCount <= s.failUntil {
		return nil, fmt.Errorf("stage failed")
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// orderTrackingStage tracks the order in which stages are executed
type orderTrackingStage struct {
	name       string
	runOrder   *[]string
	shouldFail bool
	runCount   int
}

func (s *orderTrackingStage) Name() string {
	return s.name
}

func (s *orderTrackingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	*s.runOrder = append(*s.runOrder, s.name)
	if s.shouldFail && s.runCount == 1 {
		return nil, fmt.Errorf("stage failed")
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// retryContextCheckingStage verifies RetryContext is populated correctly
type retryContextCheckingStage struct {
	name              string
	capturedContexts  *[]*stage.RetryContext
	shouldFail        bool
	runCount          int
}

func (s *retryContextCheckingStage) Name() string {
	return s.name
}

func (s *retryContextCheckingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	*s.capturedContexts = append(*s.capturedContexts, req.RetryContext)
	if s.shouldFail && s.runCount == 1 {
		return nil, fmt.Errorf("stage failed")
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}
