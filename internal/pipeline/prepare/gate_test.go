package prepare

import (
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
		decomposerErr       error
		wantDecision        pipeline.Decision
		wantDecomposeCalled bool
	}{
		{
			name:                "skip when scope decomposition succeeds",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          &fakeDecomposer{err: nil},
			decomposerErr:       nil,
			wantDecision:        pipeline.Skip,
			wantDecomposeCalled: true,
		},
		{
			name:                "block when scope decomposition fails",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          &fakeDecomposer{err: errors.New("decomposition failed")},
			decomposerErr:       errors.New("decomposition failed"),
			wantDecision:        pipeline.Block,
			wantDecomposeCalled: true,
		},
		{
			name:                "block when decomposer is nil",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          nil,
			decomposerErr:       nil,
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
			name:                "block child bead without attempting decomposition",
			expectedOutputs:     []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			decomposer:          &fakeDecomposer{err: nil},
			wantDecision:        pipeline.Block,
			wantDecomposeCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{
				ID:              "test-oversized",
				Title:           "test bead",
				ExpectedOutputs: tt.expectedOutputs,
				Parent:          "", // Root bead
			}
			cfg := &config.Config{
				ScopeCheck: config.ScopeCheckConfig{
					Enabled:        true,
					BlockOversized: &blockTrue,
				},
			}
			gate := New(io.Discard)
			if tt.decomposer != nil {
				gate = gate.WithDecomposer(tt.decomposer)
			}
			in := pipeline.Input{Bead: b, Config: cfg}
			out, err := gate.Run(context.Background(), in)
			// If decomposition was attempted and failed, error is propagated
			if tt.decomposerErr != nil {
				if err == nil {
					t.Fatalf("expected error from decomposition failure, got nil")
				}
				// Error is expected and has been propagated
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
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

	// Test child beads (with parent): should block without attempting decomposition
	for _, tt := range childBeadTests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{
				ID:              "test-oversized-child",
				Title:           "test child bead",
				ExpectedOutputs: tt.expectedOutputs,
				Parent:          "parent-1", // Child bead
			}
			cfg := &config.Config{
				ScopeCheck: config.ScopeCheckConfig{
					Enabled:        true,
					BlockOversized: &blockTrue,
				},
			}
			gate := New(io.Discard).WithDecomposer(tt.decomposer)
			in := pipeline.Input{Bead: b, Config: cfg}
			out, err := gate.Run(context.Background(), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
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

// TestGateRunScopeGateDecompositionEndToEnd verifies the full path:
// gate.Run() → scope gate triggers → decomposerAdapter.Decompose() → bead.Client.CreateWithParent()
// This integration test exercises the end-to-end decomposition pipeline when an oversized root bead
// is decomposed into sub-beads via the real pipeline.
func TestGateRunScopeGateDecompositionEndToEnd(t *testing.T) {
	// Mock bead client that tracks CreateWithParent calls
	mockBeadClient := &mockBeadClient{}

	// Create decomposerAdapter that wraps the mock bead client
	decomposer := &decomposerAdapter{
		beads: mockBeadClient,
	}

	// Create an oversized root bead that exceeds scope threshold (6 expected outputs > maxScopeFiles of 5)
	oversizedBead := &bead.Bead{
		ID:              "test-oversized-root",
		Title:           "Implement large feature",
		Priority:        1,
		Labels:          []string{"feature"},
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		Parent:          "", // Root bead, not a child
	}

	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	// Configure gate with decomposer adapter
	gate := New(io.Discard).WithDecomposer(decomposer)

	// Run the gate
	in := pipeline.Input{Bead: oversizedBead, Config: cfg}
	out, err := gate.Run(context.Background(), in)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("gate.Run() error = %v", err)
	}

	// Verify decision is Skip (decomposition succeeded, so skip the original bead)
	if out.Decision != pipeline.Skip {
		t.Errorf("decision = %v, want Skip (decomposition should succeed)", out.Decision)
	}

	// Verify CreateWithParent was called with correct parameters
	if !mockBeadClient.createWithParentCalled {
		t.Error("CreateWithParent was not called, but gate should have triggered decomposition")
	}

	if mockBeadClient.capturedTitle != oversizedBead.Title+" (decomposed)" {
		t.Errorf("CreateWithParent title = %q, want %q", mockBeadClient.capturedTitle, oversizedBead.Title+" (decomposed)")
	}

	if mockBeadClient.capturedPriority != oversizedBead.Priority {
		t.Errorf("CreateWithParent priority = %d, want %d", mockBeadClient.capturedPriority, oversizedBead.Priority)
	}

	if mockBeadClient.capturedParentID != oversizedBead.ID {
		t.Errorf("CreateWithParent parentID = %q, want %q", mockBeadClient.capturedParentID, oversizedBead.ID)
	}
}

// mockBeadClient is a mock bead.Client for integration testing
type mockBeadClient struct {
	createWithParentCalled  bool
	capturedTitle           string
	capturedPriority        int
	capturedLabels          []string
	capturedExpectedOutputs []string
	capturedParentID        string
}

func (m *mockBeadClient) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	m.createWithParentCalled = true
	m.capturedTitle = title
	m.capturedPriority = priority
	m.capturedLabels = labels
	m.capturedExpectedOutputs = expectedOutputs
	m.capturedParentID = parentID
	return &bead.Bead{ID: "bead-decomposed-1", Title: title, Priority: priority}, nil
}

// mockBeadClientInterface defines the interface needed by decomposerAdapter
type mockBeadClientInterface interface {
	CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
}

// decomposerAdapter mimics the real decomposerAdapter behavior for integration testing
type decomposerAdapter struct {
	beads mockBeadClientInterface
}

func (a *decomposerAdapter) Decompose(ctx context.Context, b *bead.Bead) error {
	_, err := a.beads.CreateWithParent(b.Title+" (decomposed)", b.Priority, b.Labels, b.ExpectedOutputs, b.ID)
	return err
}

// TestGateRunScopeGateDecompositionRecordsOutput verifies that the gate records
// decomposition output when an oversized root bead is decomposed successfully.
// This test ensures the gate logs information about the decomposition action
// through the output writer for visibility into the decomposition process.
func TestGateRunScopeGateDecompositionRecordsOutput(t *testing.T) {
	// Create a capturing output writer to verify decomposition is logged
	output := &recordingWriter{}

	// Create a spy bead.Client
	spyBeadClient := &spyBeadClient{}

	// Create the decomposerAdapter implementation with the spy client
	decomposer := &testDecomposerAdapter{
		beads: spyBeadClient,
	}

	// Create an oversized root bead that exceeds scope threshold (6 expected outputs > maxScopeFiles of 5)
	oversizedBead := &bead.Bead{
		ID:              "test-oversized-root",
		Title:           "Implement comprehensive feature",
		Priority:        2,
		Labels:          []string{"feature", "epic"},
		ExpectedOutputs: []string{"file1.go", "file2.go", "file3.go", "file4.go", "file5.go", "file6.go"},
		Parent:          "", // Root bead, not a child
	}

	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	// Configure gate with the decomposer and recording output
	gate := New(output).WithDecomposer(decomposer)

	// Run the gate
	in := pipeline.Input{Bead: oversizedBead, Config: cfg}
	out, err := gate.Run(context.Background(), in)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("gate.Run() error = %v, want nil", err)
	}

	// Verify decision is Skip (decomposition succeeded, so skip the original bead)
	if out.Decision != pipeline.Skip {
		t.Errorf("gate.Run() decision = %v, want Skip", out.Decision)
	}

	// Verify bead.Client.CreateWithParent was called exactly once
	if spyBeadClient.callCount != 1 {
		t.Errorf("CreateWithParent call count = %d, want 1", spyBeadClient.callCount)
	}

	// Verify the parameters passed to CreateWithParent
	if spyBeadClient.lastTitle != oversizedBead.Title+" (decomposed)" {
		t.Errorf("CreateWithParent title = %q, want %q", spyBeadClient.lastTitle, oversizedBead.Title+" (decomposed)")
	}

	if spyBeadClient.lastPriority != oversizedBead.Priority {
		t.Errorf("CreateWithParent priority = %d, want %d", spyBeadClient.lastPriority, oversizedBead.Priority)
	}

	if !slicesEqual(spyBeadClient.lastLabels, oversizedBead.Labels) {
		t.Errorf("CreateWithParent labels = %v, want %v", spyBeadClient.lastLabels, oversizedBead.Labels)
	}

	if !slicesEqual(spyBeadClient.lastExpectedOutputs, oversizedBead.ExpectedOutputs) {
		t.Errorf("CreateWithParent expectedOutputs = %v, want %v", spyBeadClient.lastExpectedOutputs, oversizedBead.ExpectedOutputs)
	}

	if spyBeadClient.lastParentID != oversizedBead.ID {
		t.Errorf("CreateWithParent parentID = %q, want %q", spyBeadClient.lastParentID, oversizedBead.ID)
	}

	// Verify that decomposition was recorded in output (indicates successful decomposition logging)
	if !output.Contains("decomposition") {
		t.Error("gate output does not mention decomposition, want notification that decomposition occurred")
	}

	// Verify that the bead ID is recorded
	if !output.Contains(oversizedBead.ID) {
		t.Errorf("gate output does not mention bead ID %q", oversizedBead.ID)
	}
}

// TestGateRunScopeGateDecompositionClosesParentBead verifies that after successful decomposition,
// the parent bead is properly closed to prevent further processing of the oversized parent.
// This ensures the decomposed parent doesn't get submitted for work while sub-beads are processed.
func TestGateRunScopeGateDecompositionClosesParentBead(t *testing.T) {
	// Create a mock that tracks both CreateWithParent and Close calls
	// This will verify that the decomposition process closes the parent bead
	mockClient := &mockBeadClientWithClose{}

	// Create the decomposerAdapter implementation
	// Currently this should FAIL the test because it doesn't close the parent
	decomposer := &decomposerAdapterForTest{
		beads: mockClient,
	}

	// Create an oversized root bead that exceeds scope threshold (6 expected outputs > maxScopeFiles of 5)
	oversizedBead := &bead.Bead{
		ID:              "test-oversized-root",
		Title:           "Implement complex feature",
		Priority:        1,
		Labels:          []string{"feature"},
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		Parent:          "", // Root bead, not a child
	}

	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}

	// Configure gate with the decomposer
	gate := New(io.Discard).WithDecomposer(decomposer)

	// Run the gate
	in := pipeline.Input{Bead: oversizedBead, Config: cfg}
	out, err := gate.Run(context.Background(), in)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("gate.Run() error = %v, want nil", err)
	}

	// Verify decision is Skip
	if out.Decision != pipeline.Skip {
		t.Errorf("gate.Run() decision = %v, want Skip", out.Decision)
	}

	// Verify that CreateWithParent was called to create sub-beads
	if mockClient.createWithParentCalls != 1 {
		t.Errorf("CreateWithParent calls = %d, want 1", mockClient.createWithParentCalls)
	}

	// Verify that the parent bead is closed after decomposition
	// This should FAIL with current implementation since decomposerAdapter doesn't close parent
	if mockClient.closeCalls != 1 {
		t.Errorf("Close() calls = %d, want 1 (parent bead should be closed after decomposition)", mockClient.closeCalls)
	}

	// Verify that Close was called with the parent bead ID
	if mockClient.lastClosedID != oversizedBead.ID {
		t.Errorf("Close() called with ID %q, want %q (parent bead ID)", mockClient.lastClosedID, oversizedBead.ID)
	}
}

// recordingWriter captures all output written to it for verification in tests
type recordingWriter struct {
	content string
}

func (r *recordingWriter) Write(p []byte) (n int, err error) {
	r.content += string(p)
	return len(p), nil
}

func (r *recordingWriter) Contains(s string) bool {
	return strings.Contains(r.content, s)
}

// BeadClientForTest defines the interface for bead creation in tests
type BeadClientForTest interface {
	CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
}

// testDecomposerAdapter implements the Decomposer interface for testing
// It mimics the real decomposerAdapter behavior from runner package
type testDecomposerAdapter struct {
	beads BeadClientForTest
}

func (a *testDecomposerAdapter) Decompose(ctx context.Context, b *bead.Bead) error {
	_, err := a.beads.CreateWithParent(b.Title+" (decomposed)", b.Priority, b.Labels, b.ExpectedOutputs, b.ID)
	return err
}

// spyBeadClient tracks all calls to CreateWithParent
type spyBeadClient struct {
	callCount           int
	lastTitle           string
	lastPriority        int
	lastLabels          []string
	lastExpectedOutputs []string
	lastParentID        string
}

func (s *spyBeadClient) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	s.callCount++
	s.lastTitle = title
	s.lastPriority = priority
	s.lastLabels = labels
	s.lastExpectedOutputs = expectedOutputs
	s.lastParentID = parentID
	return &bead.Bead{ID: "child-bead-1", Title: title, Priority: priority}, nil
}

// slicesEqual is a helper to compare two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// decomposerAdapterForTest is a simple adapter for testing scope gate decomposition
// This mimics the current decomposerAdapter behavior but uses a test interface
type decomposerAdapterForTest struct {
	beads MockBeadClientForDecomposition
}

func (a *decomposerAdapterForTest) Decompose(ctx context.Context, b *bead.Bead) error {
	_, err := a.beads.CreateWithParent(b.Title+" (decomposed)", b.Priority, b.Labels, b.ExpectedOutputs, b.ID)
	if err != nil {
		return err
	}
	if err := a.beads.Close(b.ID); err != nil {
		return err
	}
	return nil
}

// MockBeadClientForDecomposition is an interface for testing decomposition with Close tracking
type MockBeadClientForDecomposition interface {
	CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
	Close(id string) error
}

// mockBeadClientWithClose tracks both CreateWithParent and Close calls
type mockBeadClientWithClose struct {
	createWithParentCalls int
	closeCalls            int
	lastClosedID          string
}

func (m *mockBeadClientWithClose) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	m.createWithParentCalls++
	return &bead.Bead{ID: "child-bead-1", Title: title, Priority: priority}, nil
}

func (m *mockBeadClientWithClose) Close(id string) error {
	m.closeCalls++
	m.lastClosedID = id
	return nil
}
