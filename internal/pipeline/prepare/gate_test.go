package prepare

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
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

type fakePrechecker struct {
	done bool
	err  error
}

func (f *fakePrechecker) Check(_ context.Context, _ *bead.Bead) (bool, error) {
	return f.done, f.err
}

type fakeStuckDetector struct {
	stuck bool
	err   error
}

func (f *fakeStuckDetector) IsStuck(_ context.Context, _ *bead.Bead) (bool, error) {
	return f.stuck, f.err
}

type fakeDecomposer struct {
	err    error
	called bool
}

func (f *fakeDecomposer) Decompose(_ context.Context, _ *bead.Bead) error {
	f.called = true
	return f.err
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

func TestGateRunProactiveDecomposition(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		parent       string
		decomposeErr error
		wantDecision pipeline.Decision
		wantCalled   bool
	}{
		{
			name:         "skip keyword candidate bead with no parent",
			title:        "Refactor config loading to use interfaces",
			parent:       "",
			decomposeErr: nil,
			wantDecision: pipeline.Skip,
			wantCalled:   true,
		},
		{
			name:         "proceed keyword candidate bead with parent (child bead)",
			title:        "Refactor config validation helpers",
			parent:       "parent-1",
			decomposeErr: nil,
			wantDecision: pipeline.Proceed,
			wantCalled:   false,
		},
		{
			name:         "proceed non-keyword bead",
			title:        "Add retry count to iteration log",
			parent:       "",
			decomposeErr: nil,
			wantDecision: pipeline.Proceed,
			wantCalled:   false,
		},
		{
			name:         "proceed when decompose fails (non-blocking)",
			title:        "Refactor config loading",
			parent:       "",
			decomposeErr: errors.New("decompose failed"),
			wantDecision: pipeline.Proceed,
			wantCalled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{
				ID:     "test-1",
				Title:  tt.title,
				Parent: tt.parent,
			}
			d := &fakeDecomposer{err: tt.decomposeErr}
			gate := New(io.Discard).WithDecomposer(d)
			in := pipeline.Input{Bead: b}
			out, err := gate.Run(context.Background(), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", out.Decision, tt.wantDecision)
			}
			if d.called != tt.wantCalled {
				t.Errorf("decomposer called = %v, want %v", d.called, tt.wantCalled)
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
	var output bytes.Buffer
	d := &fakeDecomposer{}
	gate := New(&output).WithDecomposer(d)
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
	if !strings.Contains(output.String(), "attempting decomposition") {
		t.Fatalf("output %q does not include decomposition attempt log", output.String())
	}
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
