package prepare

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
	"github.com/danabrams/gromit/internal/pipeline"
)

// Test constants to avoid repetition
var (
	blockTrue  = true
	blockFalse = false
)

type scopeGateCase struct {
	expectedOutputs []string
	decomposer      Decomposer
	parentID        string
}

func runScopeGateCase(t *testing.T, tc scopeGateCase) pipeline.Output {
	t.Helper()

	b := &bead.Bead{
		ID:              "test-oversized",
		Title:           "test bead",
		ExpectedOutputs: tc.expectedOutputs,
		Parent:          tc.parentID,
	}
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	gate := New(io.Discard)
	if tc.decomposer != nil {
		gate = gate.WithDecomposer(tc.decomposer)
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b, Config: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out
}

type mockPrechecker struct {
	done bool
	err  error
}

func newMockPrechecker() *mockPrechecker {
	return &mockPrechecker{}
}

func (m *mockPrechecker) WithCheck(done bool, err error) *mockPrechecker {
	m.done = done
	m.err = err
	return m
}

func (m *mockPrechecker) Check(_ context.Context, _ *bead.Bead) (bool, error) {
	return m.done, m.err
}

// Deprecated: use newMockPrechecker() instead
type fakePrechecker struct {
	done bool
	err  error
}

func (f *fakePrechecker) Check(_ context.Context, _ *bead.Bead) (bool, error) {
	return f.done, f.err
}

type mockStuckDetector struct {
	stuck bool
	err   error
}

func newMockStuckDetector() *mockStuckDetector {
	return &mockStuckDetector{}
}

func (m *mockStuckDetector) WithIsStuck(stuck bool, err error) *mockStuckDetector {
	m.stuck = stuck
	m.err = err
	return m
}

func (m *mockStuckDetector) IsStuck(_ context.Context, _ *bead.Bead) (bool, error) {
	return m.stuck, m.err
}

// Deprecated: use newMockStuckDetector() instead
type fakeStuckDetector struct {
	stuck bool
	err   error
}

func (f *fakeStuckDetector) IsStuck(_ context.Context, _ *bead.Bead) (bool, error) {
	return f.stuck, f.err
}

type mockDecomposer struct {
	err    error
	called bool
}

func newMockDecomposer() *mockDecomposer {
	return &mockDecomposer{}
}

func (m *mockDecomposer) WithDecompose(err error) *mockDecomposer {
	m.err = err
	return m
}

func (m *mockDecomposer) WithCalled(called bool) *mockDecomposer {
	m.called = called
	return m
}

func (m *mockDecomposer) WasCalled() bool {
	return m.called
}

func (m *mockDecomposer) Decompose(_ context.Context, _ *bead.Bead) error {
	m.called = true
	return m.err
}

// Deprecated: use newMockDecomposer() instead
type fakeDecomposer struct {
	err    error
	called bool
}

func (f *fakeDecomposer) Decompose(_ context.Context, _ *bead.Bead) error {
	f.called = true
	return f.err
}

type mockDataQualityBlocker struct {
	blocked bool
	reason  string
	err     error
}

func newMockDataQualityBlocker() *mockDataQualityBlocker {
	return &mockDataQualityBlocker{}
}

func (m *mockDataQualityBlocker) WithShouldBlock(blocked bool, reason string, err error) *mockDataQualityBlocker {
	m.blocked = blocked
	m.reason = reason
	m.err = err
	return m
}

func (m *mockDataQualityBlocker) ShouldBlock(_ context.Context, _ *bead.Bead) (bool, string, error) {
	return m.blocked, m.reason, m.err
}

// Deprecated: use newMockDataQualityBlocker() instead
type fakeDataQualityBlocker struct {
	blocked bool
	reason  string
	err     error
}

func (f *fakeDataQualityBlocker) ShouldBlock(_ context.Context, _ *bead.Bead) (bool, string, error) {
	return f.blocked, f.reason, f.err
}

func TestGateRun(t *testing.T) {
	b := &bead.Bead{ID: "test-1", Title: "test bead"}

	tests := []struct {
		name         string
		precheck     Prechecker
		stuck        StuckDetector
		wantDecision pipeline.Decision
	}{
		{
			name:         "proceed when precheck says not done and not stuck",
			precheck:     &fakePrechecker{done: false},
			stuck:        &fakeStuckDetector{stuck: false},
			wantDecision: pipeline.Proceed,
		},
		{
			name:         "skip when precheck says work is already done",
			precheck:     &fakePrechecker{done: true},
			stuck:        &fakeStuckDetector{stuck: false},
			wantDecision: pipeline.Skip,
		},
		{
			name:         "block when stuck detector says threshold exceeded",
			precheck:     &fakePrechecker{done: false},
			stuck:        &fakeStuckDetector{stuck: true},
			wantDecision: pipeline.Block,
		},
		{
			name:         "skip takes priority over block when precheck passes and bead is stuck",
			precheck:     &fakePrechecker{done: true},
			stuck:        &fakeStuckDetector{stuck: true},
			wantDecision: pipeline.Skip,
		},
		{
			name:         "proceed when no precheck configured and not stuck",
			precheck:     nil,
			stuck:        &fakeStuckDetector{stuck: false},
			wantDecision: pipeline.Proceed,
		},
		{
			name:         "proceed when no stuck detector configured and precheck not done",
			precheck:     &fakePrechecker{done: false},
			stuck:        nil,
			wantDecision: pipeline.Proceed,
		},
		{
			name:         "proceed when precheck errors (non-blocking)",
			precheck:     &fakePrechecker{err: errors.New("precheck failed")},
			stuck:        &fakeStuckDetector{stuck: false},
			wantDecision: pipeline.Proceed,
		},
		{
			name:         "proceed when stuck detector errors (non-blocking)",
			precheck:     &fakePrechecker{done: false},
			stuck:        &fakeStuckDetector{err: errors.New("stuck check failed")},
			wantDecision: pipeline.Proceed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := New(io.Discard).
				WithPrechecker(tt.precheck).
				WithStuckDetector(tt.stuck)

			in := pipeline.Input{Bead: b}
			out, err := gate.Run(context.Background(), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
		})
	}
}

func TestGateRun_PrecheckErrorEmitsLogEvent(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard).WithPrechecker(newMockPrechecker().WithCheck(false, errors.New("boom")))
	gate.WithEmitter(emitter)

	_, err := gate.Run(context.Background(), pipeline.Input{Bead: &bead.Bead{ID: "log-test"}})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	select {
	case evt := <-ch:
		logEvt, ok := evt.(*events.LogEvent)
		if !ok {
			t.Fatalf("expected LogEvent, got %T", evt)
		}
		if !strings.Contains(logEvt.Message, "precheck failed") {
			t.Fatalf("unexpected log message %q", logEvt.Message)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected LogEvent to be emitted")
	}
}

func TestGateRunNilBead(t *testing.T) {
	gate := New(io.Discard).
		WithPrechecker(&fakePrechecker{done: true}).
		WithStuckDetector(&fakeStuckDetector{stuck: true})

	in := pipeline.Input{Bead: nil}
	out, err := gate.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("nil bead: decision = %v, want Proceed", out.Decision)
	}
}

func TestGateRunScopeGate(t *testing.T) {
	tests := []struct {
		name            string
		expectedOutputs []string
		cfg             *config.Config
		wantDecision    pipeline.Decision
	}{
		{
			name:            "block oversized bead when scope check enabled",
			expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			cfg: &config.Config{ScopeCheck: config.ScopeCheckConfig{
				Enabled: true, BlockOversized: &blockTrue}},
			wantDecision: pipeline.Block,
		},
		{
			name:            "proceed bead within threshold",
			expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5"},
			cfg: &config.Config{ScopeCheck: config.ScopeCheckConfig{
				Enabled: true, BlockOversized: &blockTrue}},
			wantDecision: pipeline.Proceed,
		},
		{
			name:            "proceed when scope check disabled",
			expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			cfg:             &config.Config{ScopeCheck: config.ScopeCheckConfig{Enabled: false}},
			wantDecision:    pipeline.Proceed,
		},
		{
			name:            "proceed when block_oversized false",
			expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			cfg: &config.Config{ScopeCheck: config.ScopeCheckConfig{
				Enabled: true, BlockOversized: &blockFalse}},
			wantDecision: pipeline.Proceed,
		},
		{
			name:            "proceed when no config",
			expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			cfg:             nil,
			wantDecision:    pipeline.Proceed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{ID: "test-1", Title: "test bead", ExpectedOutputs: tt.expectedOutputs}
			gate := New(io.Discard)
			in := pipeline.Input{Bead: b, Config: tt.cfg}
			out, err := gate.Run(context.Background(), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
		})
	}
}

func TestGateScopeEventEmittedOnBlock(t *testing.T) {
	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	b := &bead.Bead{
		ID:              "test-oversized",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	gate := New(io.Discard)
	_, err := gate.Run(context.Background(), pipeline.Input{Bead: b, Config: cfg, Emitter: emitter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emittedEvents := eventtest.DrainEvents(t, ch, 100*time.Millisecond)

	var scopeEvt *events.GateScopeEvent
	for _, evt := range emittedEvents {
		if se, ok := evt.(*events.GateScopeEvent); ok {
			scopeEvt = se
		}
	}
	if scopeEvt == nil {
		t.Fatal("expected GateScopeEvent")
	}

	if scopeEvt.BeadID != b.ID {
		t.Errorf("bead id = %q, want %q", scopeEvt.BeadID, b.ID)
	}
	if scopeEvt.FileCount != len(b.ExpectedOutputs) {
		t.Errorf("file count = %d, want %d", scopeEvt.FileCount, len(b.ExpectedOutputs))
	}
	if scopeEvt.MaxFiles != maxScopeFiles {
		t.Errorf("max files = %d, want %d", scopeEvt.MaxFiles, maxScopeFiles)
	}
	if scopeEvt.Action != "block" {
		t.Errorf("action = %q, want %q", scopeEvt.Action, "block")
	}
}

func TestGateRunDoesNotProactivelyDecomposeFromKeywords(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		parent       string
		wantDecision pipeline.Decision
	}{
		{
			name:         "proceed keyword candidate bead with no parent",
			title:        "Refactor config loading to use interfaces",
			parent:       "",
			wantDecision: pipeline.Proceed,
		},
		{
			name:         "proceed keyword candidate bead with parent (child bead)",
			title:        "Refactor config validation helpers",
			parent:       "parent-1",
			wantDecision: pipeline.Proceed,
		},
		{
			name:         "proceed non-keyword bead",
			title:        "Add retry count to iteration log",
			parent:       "",
			wantDecision: pipeline.Proceed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{
				ID:     "test-1",
				Title:  tt.title,
				Parent: tt.parent,
			}
			d := &fakeDecomposer{}
			gate := New(io.Discard).WithDecomposer(d)
			in := pipeline.Input{Bead: b}
			out, err := gate.Run(context.Background(), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
			if d.called {
				t.Errorf("decomposer called = true, want false (keyword-based proactive decomposition disabled)")
			}
		})
	}
}

func TestGateRunScopeGateAttemptsDecomposition(t *testing.T) {
	tests := []struct {
		name                string
		expectedOutputs     []string
		decomposer          Decomposer
		wantDecision        pipeline.Decision
		wantDecomposeCalled bool
	}{
		{
			name:                "skip when scope decomposition succeeds",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          &fakeDecomposer{err: nil},
			wantDecision:        pipeline.Skip,
			wantDecomposeCalled: true,
		},
		{
			name:                "block when scope decomposition fails",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          &fakeDecomposer{err: errors.New("decomposition failed")},
			wantDecision:        pipeline.Block,
			wantDecomposeCalled: true,
		},
		{
			name:                "block when decomposer is nil",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          nil,
			wantDecision:        pipeline.Block,
			wantDecomposeCalled: false,
		},
	}

	childBeadTests := []struct {
		name                string
		expectedOutputs     []string
		decomposer          Decomposer
		wantDecision        pipeline.Decision
		wantDecomposeCalled bool
	}{
		{
			name:                "decompose oversized child bead",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          &fakeDecomposer{err: nil},
			wantDecision:        pipeline.Skip,
			wantDecomposeCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runScopeGateCase(t, scopeGateCase{
				expectedOutputs: tt.expectedOutputs,
				decomposer:      tt.decomposer,
			})
			if out.Decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
			if tt.decomposer != nil {
				if d, ok := tt.decomposer.(*fakeDecomposer); ok {
					if d.called != tt.wantDecomposeCalled {
						t.Errorf("decomposer called = %v, want %v", d.called, tt.wantDecomposeCalled)
					}
				}
			}
		})
	}

	// Test child beads (with parent): oversized children get decomposed too
	for _, tt := range childBeadTests {
		t.Run(tt.name, func(t *testing.T) {
			out := runScopeGateCase(t, scopeGateCase{
				expectedOutputs: tt.expectedOutputs,
				decomposer:      tt.decomposer,
				parentID:        "parent-1",
			})
			if out.Decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
			if d, ok := tt.decomposer.(*fakeDecomposer); ok {
				if d.called != tt.wantDecomposeCalled {
					t.Errorf("decomposer called = %v, want %v", d.called, tt.wantDecomposeCalled)
				}
			}
		})
	}
}

func TestGateRunScopeGateBehavior_WithSharedCaseHelper(t *testing.T) {
	d := &fakeDecomposer{}
	out := runScopeGateCase(t, scopeGateCase{
		expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		decomposer:      d,
	})

	if out.Decision != pipeline.Skip {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
	}
	if !d.called {
		t.Fatal("decomposer called = false, want true")
	}
}

func TestGateRunScopeGateBehavior_WithSharedCaseHelperOnChildBead(t *testing.T) {
	d := &fakeDecomposer{}
	out := runScopeGateCase(t, scopeGateCase{
		expectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		decomposer:      d,
		parentID:        "parent-1",
	})

	if out.Decision != pipeline.Skip {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
	}
	if !d.called {
		t.Fatal("decomposer called = false, want true")
	}
}

func TestGateRunScopeGateLogsWhenAttemptingDecomposition(t *testing.T) {
	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	d := &fakeDecomposer{}
	gate := New(io.Discard).WithDecomposer(d).WithEmitter(emitter)
	b := &bead.Bead{
		ID:              "bead-1",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b, Config: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Decision != pipeline.Skip {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
	}
	if !d.called {
		t.Fatalf("decomposer called = false, want true")
	}

	messages := collectLogMessages(t, ch, 2)
	if !anyContains(messages, "attempting decomposition") {
		t.Fatalf("messages %q do not include decomposition attempt log", messages)
	}
	if !anyContains(messages, "decomposition succeeded") {
		t.Fatalf("messages %q do not include decomposition success log", messages)
	}
}

func collectLogMessages(t *testing.T, ch <-chan events.Event, count int) []string {
	t.Helper()
	messages := make([]string, 0, count)
	deadline := time.After(100 * time.Millisecond)
	for len(messages) < count {
		select {
		case evt := <-ch:
			if logEvt, ok := evt.(*events.LogEvent); ok {
				messages = append(messages, logEvt.Message)
			}
		case <-deadline:
			t.Fatalf("expected %d LogEvent(s), got %d", count, len(messages))
		}
	}
	return messages
}

func anyContains(messages []string, substr string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}

// TestGateScopeDecompositionErrorFallsBackToBlock verifies that when the LLM-powered
// decomposer returns an error, the scope gate falls back to blocking the bead
// rather than propagating the error up to the caller.
func TestGateScopeDecompositionErrorFallsBackToBlock(t *testing.T) {
	b := &bead.Bead{
		ID:              "test-oversized",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}
	d := &fakeDecomposer{err: errors.New("llm decomposition failed")}
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}
	gate := New(io.Discard).WithDecomposer(d)
	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b, Config: cfg})
	if err != nil {
		t.Fatalf("unexpected error %v; want nil (decomposition failure should fall back to Block, not propagate error)", err)
	}
	if out.Decision != pipeline.Block {
		t.Errorf("decision = %v, want Block when LLM decomposition fails", out.Decision)
	}
}

func TestGateRunComplexityRouting(t *testing.T) {
	tests := []struct {
		name         string
		in           pipeline.Input
		wantDecision pipeline.Decision
		wantComplex  string
		wantSource   string
		wantReason   string
	}{
		{
			name: "scope estimate complexity wins over complexity label and is normalized",
			in: pipeline.Input{
				Bead: &bead.Bead{
					ID:     "bead-1",
					Title:  "test",
					Labels: []string{"complexity:low"},
				},
				ComplexityRouting: pipeline.ComplexityRouting{
					Complexity: " HIGH ",
				},
			},
			wantDecision: pipeline.Proceed,
			wantComplex:  "high",
			wantSource:   "scope_estimate",
			wantReason:   "none",
		},
		{
			name: "complexity label is used when scope estimate complexity is unavailable",
			in: pipeline.Input{
				Bead: &bead.Bead{
					ID:     "bead-2",
					Title:  "test",
					Labels: []string{"complexity: HIGH "},
				},
			},
			wantDecision: pipeline.Proceed,
			wantComplex:  "high",
			wantSource:   "label",
			wantReason:   "scope_unavailable",
		},
		{
			name: "falls back to medium with explicit reason when scope and label are unavailable",
			in: pipeline.Input{
				Bead: &bead.Bead{
					ID:     "bead-3",
					Title:  "test",
					Labels: []string{"priority:p1"},
				},
				ComplexityRouting: pipeline.ComplexityRouting{
					Complexity: "   ",
				},
			},
			wantDecision: pipeline.Proceed,
			wantComplex:  "medium",
			wantSource:   "default",
			wantReason:   "scope_and_label_unavailable",
		},
	}

	gate := New(io.Discard)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := gate.Run(context.Background(), tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Decision != tt.wantDecision {
				t.Fatalf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
			if out.Complexity != tt.wantComplex {
				t.Errorf("complexity = %q, want %q", out.Complexity, tt.wantComplex)
			}
			if out.ComplexitySource != tt.wantSource {
				t.Errorf("complexity source = %q, want %q", out.ComplexitySource, tt.wantSource)
			}
			if out.ComplexityFallbackReason != tt.wantReason {
				t.Errorf("complexity fallback reason = %q, want %q", out.ComplexityFallbackReason, tt.wantReason)
			}
		})
	}
}

func TestGateRunComplexityRouting_OnSkipAndBlockPaths(t *testing.T) {
	b := &bead.Bead{
		ID:     "bead-1",
		Title:  "test",
		Labels: []string{"complexity:high"},
	}
	in := pipeline.Input{Bead: b}

	t.Run("precheck skip includes complexity routing", func(t *testing.T) {
		gate := New(io.Discard).WithPrechecker(&fakePrechecker{done: true})
		out, err := gate.Run(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Decision != pipeline.Skip {
			t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
		}
		if out.Complexity != "high" || out.ComplexitySource != "label" || out.ComplexityFallbackReason != "scope_unavailable" {
			t.Fatalf("unexpected complexity routing: %+v", out.ComplexityRouting)
		}
	})

	t.Run("stuck block includes complexity routing", func(t *testing.T) {
		gate := New(io.Discard).WithStuckDetector(&fakeStuckDetector{stuck: true})
		out, err := gate.Run(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Decision != pipeline.Block {
			t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
		}
		if out.Complexity != "high" || out.ComplexitySource != "label" || out.ComplexityFallbackReason != "scope_unavailable" {
			t.Fatalf("unexpected complexity routing: %+v", out.ComplexityRouting)
		}
	})
}

func TestGateRunDataQualityBlocker_BlocksBeadWhenDataIncomplete(t *testing.T) {
	b := &bead.Bead{
		ID:    "bead-quality-check",
		Title: "test bead",
	}
	in := pipeline.Input{Bead: b}

	gate := New(io.Discard).WithDataQualityBlocker(&fakeDataQualityBlocker{blocked: true, reason: "incomplete_efficiency_data"})
	out, err := gate.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}
}

func TestConsolidatedMocks_PrecheckerCanBeCreatedWithHelper(t *testing.T) {
	p := newMockPrechecker().WithCheck(false, nil)

	done, err := p.Check(context.Background(), &bead.Bead{ID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done != false {
		t.Errorf("done = %v, want false", done)
	}
}

func TestConsolidatedMocks_StuckDetectorCanBeCreatedWithHelper(t *testing.T) {
	s := newMockStuckDetector().WithIsStuck(true, nil)

	stuck, err := s.IsStuck(context.Background(), &bead.Bead{ID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stuck != true {
		t.Errorf("stuck = %v, want true", stuck)
	}
}

func TestConsolidatedMocks_DecomposerCanBeCreatedWithHelper(t *testing.T) {
	d := newMockDecomposer().WithDecompose(nil).WithCalled(false)

	err := d.Decompose(context.Background(), &bead.Bead{ID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.WasCalled() {
		t.Error("WasCalled() = false, want true")
	}
}

func TestConsolidatedMocks_DataQualityBlockerCanBeCreatedWithHelper(t *testing.T) {
	dq := newMockDataQualityBlocker().WithShouldBlock(true, "incomplete_data", nil)

	blocked, reason, err := dq.ShouldBlock(context.Background(), &bead.Bead{ID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Errorf("blocked = %v, want true", blocked)
	}
	if reason != "incomplete_data" {
		t.Errorf("reason = %q, want %q", reason, "incomplete_data")
	}
}

// RED: test for gate outcomes emitting typed events
func TestGateRun_SkipEmitsGateSkipEvent(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard).WithPrechecker(newMockPrechecker().WithCheck(true, nil))

	beadID := "skip-test"
	_, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    &bead.Bead{ID: beadID, Title: "test bead"},
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	select {
	case evt := <-ch:
		skipEvt, ok := evt.(*events.GateSkipEvent)
		if !ok {
			t.Fatalf("expected GateSkipEvent, got %T", evt)
		}
		if skipEvt.BeadID != beadID {
			t.Errorf("BeadID = %q, want %q", skipEvt.BeadID, beadID)
		}
		if skipEvt.Reason != "precheck_passed" {
			t.Errorf("Reason = %q, want %q", skipEvt.Reason, "precheck_passed")
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected GateSkipEvent to be emitted")
	}
}

// RED: test for GateStuckEvent when stuck detector identifies stuck bead
func TestGateRun_StuckEmitsGateStuckEvent(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard).WithStuckDetector(newMockStuckDetector().WithIsStuck(true, nil))

	beadID := "stuck-test"
	_, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    &bead.Bead{ID: beadID, Title: "stuck bead"},
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	select {
	case evt := <-ch:
		stuckEvt, ok := evt.(*events.GateStuckEvent)
		if !ok {
			t.Fatalf("expected GateStuckEvent, got %T", evt)
		}
		if stuckEvt.BeadID != beadID {
			t.Errorf("BeadID = %q, want %q", stuckEvt.BeadID, beadID)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected GateStuckEvent to be emitted")
	}
}

// RED: test for GateBlockEvent when scope gate blocks oversized bead
func TestGateRun_ScopeBlockEmitsGateBlockEvent(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard)

	beadID := "scope-block-test"
	b := &bead.Bead{
		ID:              beadID,
		Title:           "oversized bead",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"}, // exceeds maxScopeFiles (5)
	}
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	_, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    b,
		Config:  cfg,
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	emittedEvents := eventtest.DrainEvents(t, ch)

	var blockEvt *events.GateBlockEvent
	for _, evt := range emittedEvents {
		if be, ok := evt.(*events.GateBlockEvent); ok {
			blockEvt = be
		}
	}
	if blockEvt == nil {
		t.Fatal("expected GateBlockEvent to be emitted")
	}
	if blockEvt.BeadID != beadID {
		t.Errorf("BeadID = %q, want %q", blockEvt.BeadID, beadID)
	}
	if blockEvt.Reason != "scope" {
		t.Errorf("Reason = %q, want %q", blockEvt.Reason, "scope")
	}
}

// RED: test for ReadinessBlocker blocking bead and emitting BlockEvent with reason code
func TestGateRun_ReadinessBlockEmitsGateBlockEventWithReason(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard).WithDataQualityBlocker(
		newMockDataQualityBlocker().WithShouldBlock(true, "criteria_missing", nil),
	)

	beadID := "readiness-block-test"
	b := &bead.Bead{
		ID:    beadID,
		Title: "test bead",
	}

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    b,
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}

	emittedEvents := eventtest.DrainEvents(t, ch)

	var blockEvt *events.GateBlockEvent
	for _, evt := range emittedEvents {
		if be, ok := evt.(*events.GateBlockEvent); ok {
			blockEvt = be
		}
	}
	if blockEvt == nil {
		t.Fatal("expected GateBlockEvent to be emitted")
	}
	if blockEvt.BeadID != beadID {
		t.Errorf("BeadID = %q, want %q", blockEvt.BeadID, beadID)
	}
	if blockEvt.Reason != "criteria_missing" {
		t.Errorf("Reason = %q, want %q", blockEvt.Reason, "criteria_missing")
	}
}
