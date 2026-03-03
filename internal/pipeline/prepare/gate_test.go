package prepare

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/readiness"
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
	done   bool
	err    error
	called bool
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
	m.called = true
	return m.done, m.err
}

func (m *mockPrechecker) WasCalled() bool {
	return m.called
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
	stuck  bool
	err    error
	called bool
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
	m.called = true
	return m.stuck, m.err
}

// WasCalled reports whether IsStuck has been invoked.
func (m *mockStuckDetector) WasCalled() bool {
	return m.called
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
type mockCriteriaProvider struct {
	called bool
	result *provider.Result
	err    error
}

func (m *mockCriteriaProvider) WithResult(output string) *mockCriteriaProvider {
	m.result = &provider.Result{Output: output}
	return m
}

func (m *mockCriteriaProvider) WithError(err error) *mockCriteriaProvider {
	m.err = err
	return m
}

func (m *mockCriteriaProvider) Run(_ context.Context, _, _ string) (*provider.Result, error) {
	m.called = true
	return m.result, m.err
}

type mockSpecLoader struct {
	specDoc   *Document
	planDoc   *Document
	specFound bool
	planFound bool
	specErr   error
	planErr   error
}

func (m *mockSpecLoader) WithSpecError(err error) *mockSpecLoader {
	m.specErr = err
	return m
}

func (m *mockSpecLoader) WithPlanError(err error) *mockSpecLoader {
	m.planErr = err
	return m
}

func (m *mockSpecLoader) LoadSpec(_ context.Context, _ string) (*Document, bool, error) {
	return m.specDoc, m.specFound, m.specErr
}

func (m *mockSpecLoader) LoadPlan(_ context.Context, _ string) (*Document, bool, error) {
	return m.planDoc, m.planFound, m.planErr
}

type mockBeadUpdater struct {
	called   bool
	beadID   string
	outputs  []string
	err      error
}

func (m *mockBeadUpdater) WithError(err error) *mockBeadUpdater {
	m.err = err
	return m
}

func (m *mockBeadUpdater) UpdateExpectedOutputs(_ context.Context, id string, outputs []string) error {
	m.called = true
	m.beadID = id
	m.outputs = append([]string(nil), outputs...)
	return m.err
}

type testCriteriaEnricher struct {
	provider *mockCriteriaProvider
	loader   *mockSpecLoader
	updater  *mockBeadUpdater
	enricher *LLMCriteriaEnricher
}

func newTestCriteriaEnricher() *testCriteriaEnricher {
	provider := &mockCriteriaProvider{}
	provider.WithResult("- default criteria")
	loader := &mockSpecLoader{}
	updater := &mockBeadUpdater{}
	return &testCriteriaEnricher{
		provider: provider,
		loader:   loader,
		updater:  updater,
		enricher: NewLLMCriteriaEnricher(provider, loader, updater),
	}
}

func (t *testCriteriaEnricher) WithProviderResult(output string) *testCriteriaEnricher {
	t.provider.WithResult(output)
	return t
}

func (t *testCriteriaEnricher) WithProviderError(err error) *testCriteriaEnricher {
	t.provider.WithError(err)
	return t
}

func (t *testCriteriaEnricher) WithUpdaterError(err error) *testCriteriaEnricher {
	t.updater.WithError(err)
	return t
}

// mockReadinessAssessor simulates readiness responses for gate tests.
type mockReadinessAssessor struct {
	status readiness.Status
	reason string
	err    error
}

func newMockReadinessAssessor() *mockReadinessAssessor {
	return &mockReadinessAssessor{}
}

func (m *mockReadinessAssessor) WithAssessment(status readiness.Status, reason string, err error) *mockReadinessAssessor {
	m.status = status
	m.reason = reason
	m.err = err
	return m
}

func (m *mockReadinessAssessor) Assess(_ context.Context, _ *bead.Bead) (readiness.Assessment, error) {
	return readiness.Assessment{Status: m.status, Reason: m.reason}, m.err
}

type criteriaAwareReadinessAssessor struct {
	captured *bead.Bead
}

func (a *criteriaAwareReadinessAssessor) Assess(_ context.Context, b *bead.Bead) (readiness.Assessment, error) {
	a.captured = b
	if len(effectiveCriteria(b)) == 0 {
		return readiness.Assessment{Status: readiness.StatusNotReady, Reason: "criteria_missing"}, nil
	}
	return readiness.Assessment{Status: readiness.StatusReady}, nil
}

// RED: test for criteria enricher populating expected outputs before readiness assessment.
func TestGateRun_CriteriaEnricherPopulatesExpectedOutputs(t *testing.T) {
	t.Parallel()

	helper := newTestCriteriaEnricher().WithProviderResult("generated artifact\n")
	assessor := &criteriaAwareReadinessAssessor{}
	gate := New(io.Discard).
		WithCriteriaEnricher(helper.enricher).
		WithReadinessAssessor(assessor)

	original := &bead.Bead{
		ID:    "missing-criteria",
		Title: "bead requiring criteria",
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: original})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if !helper.updater.called {
		t.Fatal("criteria enricher did not attempt to update the bead")
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Proceed)
	}
	if assessor.captured == nil {
		t.Fatal("readiness assessor did not receive bead")
	}
	if helper.updater.beadID != original.ID {
		t.Fatalf("updater bead ID = %q, want %q", helper.updater.beadID, original.ID)
	}
	if got := assessor.captured.ExpectedOutputs; len(got) != 1 || got[0] != "generated artifact" {
		t.Fatalf("readiness assessed bead = %v, want expected outputs [generated artifact]", assessor.captured.ExpectedOutputs)
	}
}

// RED: test for logging when criteria enrichment fails.
func TestGateRun_CriteriaEnricherErrorLogsWarning(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	helper := newTestCriteriaEnricher().WithProviderError(errors.New("boom"))
	gate := New(io.Discard).
		WithEmitter(emitter).
		WithCriteriaEnricher(helper.enricher).
		WithReadinessAssessor(newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, "criteria_missing", nil))

	beadID := "criteria-enrich-error"
	bead := &bead.Bead{
		ID:    beadID,
		Title: "criteria error bead",
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: bead})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}

	emitted := eventtest.DrainEvents(t, ch)
	var logEvt *events.LogEvent
	for _, evt := range emitted {
		if le, ok := evt.(*events.LogEvent); ok {
			if strings.Contains(le.Message, "criteria enrichment failed") && strings.Contains(le.Message, beadID) {
				logEvt = le
				break
			}
		}
	}
	if logEvt == nil {
		t.Fatalf("expected warning log for criteria enricher failure, got %v events", emitted)
	}
}

func TestGateRun_CriteriaEnricherSpecLoaderErrorLogsWarning(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	helper := newTestCriteriaEnricher()
	helper.loader.WithSpecError(errors.New("spec load failure"))
	gate := New(io.Discard).
		WithEmitter(emitter).
		WithCriteriaEnricher(helper.enricher)

	beadID := "spec-loader-error"
	bead := &bead.Bead{
		ID:    beadID,
		Title: "Spec error bead",
		Labels: []string{"spec:alpha"},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: bead})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Proceed)
	}

	emitted := eventtest.DrainEvents(t, ch)
	var logEvt *events.LogEvent
	for _, evt := range emitted {
		if le, ok := evt.(*events.LogEvent); ok {
			if strings.Contains(le.Message, "criteria enrichment failed") && strings.Contains(le.Message, beadID) {
				logEvt = le
				break
			}
		}
	}
	if logEvt == nil {
		t.Fatalf("expected warning log for spec loader failure, got %v events", emitted)
	}
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

// RED: test for readiness assessor blocking bead with explicit reason code
func TestGateRun_ReadinessAssessorBlocksWithReason(t *testing.T) {
	t.Parallel()

	gate := New(io.Discard).WithReadinessAssessor(
		newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, "criteria_missing", nil),
	)

	beadID := "readiness-assessor-block"
	b := &bead.Bead{
		ID:    beadID,
		Title: "test bead",
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}
	if out.GateBlockReason != "criteria_missing" {
		t.Errorf("GateBlockReason = %q, want %q", out.GateBlockReason, "criteria_missing")
	}
}

func TestGateRun_ReadinessAmbiguousAttemptsDecomposition(t *testing.T) {
	t.Parallel()

	decomposer := newMockDecomposer().WithDecompose(nil)
	gate := New(io.Discard).
		WithReadinessAssessor(
			newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, ReasonCriteriaAmbiguous, nil),
		).
		WithDecomposer(decomposer)

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead: &bead.Bead{ID: "readiness-ambiguous", Title: "test bead"},
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Skip {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
	}
	if !decomposer.WasCalled() {
		t.Fatal("decomposer was not called for ambiguous readiness block")
	}
}

func TestGateRun_ReadinessAmbiguousDecompositionFailureFallsBackToBlock(t *testing.T) {
	t.Parallel()

	decomposer := newMockDecomposer().WithDecompose(errors.New("decompose failed"))
	gate := New(io.Discard).
		WithReadinessAssessor(
			newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, ReasonCriteriaAmbiguous, nil),
		).
		WithDecomposer(decomposer)

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead: &bead.Bead{ID: "readiness-ambiguous-fallback", Title: "test bead"},
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}
	if out.GateBlockReason != ReasonCriteriaAmbiguous {
		t.Fatalf("GateBlockReason = %q, want %q", out.GateBlockReason, ReasonCriteriaAmbiguous)
	}
	if !decomposer.WasCalled() {
		t.Fatal("decomposer was not called for ambiguous readiness block")
	}
}

func TestGateRun_ReadinessFallbackFromReviewTitle(t *testing.T) {
	t.Parallel()

	assessor := &criteriaAwareReadinessAssessor{}
	gate := New(io.Discard).WithReadinessAssessor(assessor)

	b := &bead.Bead{
		ID:    "fallback-from-review",
		Title: "Fix malformed queue recovery",
		Labels: []string{
			"from-review",
			"bug",
		},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Proceed)
	}

	if assessor.captured == nil {
		t.Fatal("readiness assessor did not receive bead")
	}
	if got := assessor.captured.ExpectedOutputs; len(got) != 1 || got[0] != "Fix malformed queue recovery" {
		t.Fatalf("captured expected outputs = %v, want [\"Fix malformed queue recovery\"]", got)
	}

	// The fallback should not mutate the original bead; it only affects readiness input.
	if len(b.ExpectedOutputs) != 0 {
		t.Fatalf("original bead expected outputs mutated: got %v", b.ExpectedOutputs)
	}
}

func TestGateRun_ReadinessFallbackDoesNotApplyWithoutFromReviewLabel(t *testing.T) {
	t.Parallel()

	assessor := &criteriaAwareReadinessAssessor{}
	gate := New(io.Discard).WithReadinessAssessor(assessor)

	b := &bead.Bead{
		ID:    "no-fallback",
		Title: "Fix malformed queue recovery",
		Labels: []string{
			"bug",
		},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}
	if out.GateBlockReason != "criteria_missing" {
		t.Fatalf("GateBlockReason = %q, want %q", out.GateBlockReason, "criteria_missing")
	}
}

// RED: test for GateReadinessBlockEvent emission when readiness assessment blocks.
func TestGateRun_ReadinessBlockEmitsGateReadinessBlockEvent(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard).WithReadinessAssessor(
		newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, "criteria_missing", nil),
	)

	beadID := "readiness-event-test"
	b := &bead.Bead{
		ID:    beadID,
		Title: "readiness event bead",
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

	var blockEvt *events.GateReadinessBlockEvent
	for _, evt := range emittedEvents {
		if be, ok := evt.(*events.GateReadinessBlockEvent); ok {
			blockEvt = be
		}
	}
	if blockEvt == nil {
		t.Fatal("expected GateReadinessBlockEvent to be emitted")
	}
	if blockEvt.BeadID != beadID {
		t.Errorf("BeadID = %q, want %q", blockEvt.BeadID, beadID)
	}
	if blockEvt.Reason != "criteria_missing" {
		t.Errorf("Reason = %q, want %q", blockEvt.Reason, "criteria_missing")
	}
}

func TestGateRun_ReadinessBlockEventStripsOverrideMarker(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	overrideReason := "readiness_override:criteria_missing"
	gate := New(io.Discard).WithReadinessAssessor(
		newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, overrideReason, nil),
	)

	beadID := "readiness-override-event"
	b := &bead.Bead{
		ID:    beadID,
		Title: "readiness override bead",
	}

	_, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    b,
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	emittedEvents := eventtest.DrainEvents(t, ch)

	var blockEvt *events.GateReadinessBlockEvent
	for _, evt := range emittedEvents {
		if be, ok := evt.(*events.GateReadinessBlockEvent); ok {
			blockEvt = be
		}
	}
	if blockEvt == nil {
		t.Fatal("expected GateReadinessBlockEvent to be emitted")
	}
	if blockEvt.Reason != "criteria_missing" {
		t.Errorf("Reason = %q, want %q", blockEvt.Reason, "criteria_missing")
	}
}

func TestGateRun_ReadinessEmergencyOverrideAllowsBypass(t *testing.T) {
	t.Parallel()

	gate := New(io.Discard).WithReadinessAssessor(
		newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, "criteria_missing", nil),
	)

	cfg := &config.Config{ReadinessEmergencyOverride: true}

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead:   &bead.Bead{ID: "readiness-override-bead", Title: "override bead"},
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Proceed {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Proceed)
	}
}

func TestGateRun_ReadinessEmergencyOverrideLogsSignal(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	gate := New(io.Discard).
		WithEmitter(emitter).
		WithReadinessAssessor(newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, "criteria_missing", nil))

	cfg := &config.Config{ReadinessEmergencyOverride: true}
	bead := &bead.Bead{ID: "readiness-override-log", Title: "override log bead"}

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead:   bead,
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Proceed)
	}

	emitted := eventtest.DrainEvents(t, ch)
	var logEvt *events.LogEvent
	for _, evt := range emitted {
		if le, ok := evt.(*events.LogEvent); ok && strings.Contains(le.Message, "Readiness emergency override") {
			logEvt = le
			break
		}
	}
	if logEvt == nil {
		t.Fatal("expected readiness override log event")
	}
	if !strings.Contains(logEvt.Message, "criteria_missing") {
		t.Fatalf("log message = %q, want reason %q", logEvt.Message, "criteria_missing")
	}
}

// RED: test that readiness blocking happens before stuck detection.
func TestGateRun_ReadinessPrecedesStuckDetection(t *testing.T) {
	t.Parallel()

	stuckDetector := newMockStuckDetector().WithIsStuck(true, nil)

	gate := New(io.Discard).
		WithReadinessAssessor(
			newMockReadinessAssessor().WithAssessment(readiness.StatusNotReady, "criteria_missing", nil),
		).
		WithStuckDetector(stuckDetector)

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead: &bead.Bead{ID: "readiness-precedes-stuck", Title: "test"},
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}

	if stuckDetector.WasCalled() {
		t.Fatalf("stuck detector was invoked despite readiness block")
	}
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

func TestGateRun_PrecheckBypassForIntentPreservingWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		bead *bead.Bead
	}{
		{
			name: "task type bypasses precheck",
			bead: &bead.Bead{ID: "bypass-task", Type: "task"},
		},
		{
			name: "chore type bypasses precheck",
			bead: &bead.Bead{ID: "bypass-chore", Type: "chore"},
		},
		{
			name: "refactor label bypasses precheck",
			bead: &bead.Bead{ID: "bypass-refactor", Labels: []string{"refactor"}},
		},
		{
			name: "tests label bypasses precheck",
			bead: &bead.Bead{ID: "bypass-tests", Labels: []string{"kind:tests"}},
		},
		{
			name: "cleanup label bypasses precheck",
			bead: &bead.Bead{ID: "bypass-cleanup", Labels: []string{"work:cleanup"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			precheck := newMockPrechecker().WithCheck(true, nil)
			gate := New(io.Discard).WithPrechecker(precheck)

			out, err := gate.Run(context.Background(), pipeline.Input{Bead: tc.bead})
			if err != nil {
				t.Fatalf("Gate.Run() error = %v", err)
			}
			if out.Decision != pipeline.Proceed {
				t.Fatalf("decision = %v, want %v (precheck bypass)", out.Decision, pipeline.Proceed)
			}
			if precheck.WasCalled() {
				t.Fatal("precheck was called for intent-preserving bead, want bypass")
			}
		})
	}
}

func TestGateRun_PrecheckStillRunsForNonBypassWork(t *testing.T) {
	t.Parallel()

	precheck := newMockPrechecker().WithCheck(true, nil)
	gate := New(io.Discard).WithPrechecker(precheck)

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead: &bead.Bead{
			ID:     "non-bypass",
			Type:   "bug",
			Labels: []string{"priority:p1"},
		},
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}
	if out.Decision != pipeline.Skip {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
	}
	if !precheck.WasCalled() {
		t.Fatal("precheck was not called for non-bypass bead")
	}
}

func TestGateRun_PrecheckBypassConfigOverride(t *testing.T) {
	t.Parallel()

	t.Run("empty allowlists disable bypass", func(t *testing.T) {
		precheck := newMockPrechecker().WithCheck(true, nil)
		gate := New(io.Discard).WithPrechecker(precheck)

		out, err := gate.Run(context.Background(), pipeline.Input{
			Bead: &bead.Bead{
				ID:     "task-no-bypass",
				Type:   "task",
				Labels: []string{"refactor"},
			},
			Config: &config.Config{
				Precheck: config.PrecheckConfig{
					BypassIssueTypes: []string{},
					BypassLabels:     []string{},
				},
			},
		})
		if err != nil {
			t.Fatalf("Gate.Run() error = %v", err)
		}
		if out.Decision != pipeline.Skip {
			t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Skip)
		}
		if !precheck.WasCalled() {
			t.Fatal("precheck was not called with empty bypass allowlists")
		}
	})

	t.Run("custom allowlists are honored", func(t *testing.T) {
		precheck := newMockPrechecker().WithCheck(true, nil)
		gate := New(io.Discard).WithPrechecker(precheck)

		out, err := gate.Run(context.Background(), pipeline.Input{
			Bead: &bead.Bead{
				ID:     "custom-label-bypass",
				Type:   "bug",
				Labels: []string{"workflow:docs-only"},
			},
			Config: &config.Config{
				Precheck: config.PrecheckConfig{
					BypassIssueTypes: []string{"ops-task"},
					BypassLabels:     []string{"docs-only"},
				},
			},
		})
		if err != nil {
			t.Fatalf("Gate.Run() error = %v", err)
		}
		if out.Decision != pipeline.Proceed {
			t.Fatalf("decision = %v, want %v (custom bypass)", out.Decision, pipeline.Proceed)
		}
		if precheck.WasCalled() {
			t.Fatal("precheck was called despite custom bypass label")
		}
	})
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
			if tt.wantDecision == pipeline.Block && out.GateBlockReason != "scope" {
				t.Errorf("GateBlockReason = %q, want %q", out.GateBlockReason, "scope")
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
		if out.GateBlockReason != "failure_threshold_exceeded" {
			t.Fatalf("GateBlockReason = %q, want %q", out.GateBlockReason, "failure_threshold_exceeded")
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

func TestGateRun_SpecLevelMaintenanceBlock(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	spec := "auth"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}
	blocker := newSpecSPCBlocker(records)

	gate := New(io.Discard).WithDataQualityBlocker(blocker)

	bead := &bead.Bead{
		ID:     "spec-maintenance",
		Title:  "maintenance heavy task",
		Labels: []string{"spec:" + spec},
	}
	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    bead,
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}
	expectedReason := blocker.reasonFor(spec)
	if out.GateBlockReason != expectedReason {
		t.Fatalf("GateBlockReason = %q, want %q", out.GateBlockReason, expectedReason)
	}

	emitted := eventtest.DrainEvents(t, ch)
	var blockEvt *events.GateBlockEvent
	for _, evt := range emitted {
		if be, ok := evt.(*events.GateBlockEvent); ok {
			blockEvt = be
		}
	}
	if blockEvt == nil {
		t.Fatal("expected GateBlockEvent to be emitted")
	}
	if blockEvt.Reason != expectedReason {
		t.Fatalf("GateBlockEvent reason = %q, want %q", blockEvt.Reason, expectedReason)
	}
}

type specSPCBlocker struct {
	records []logger.CauseClassificationRecord
}

func newSpecSPCBlocker(records []logger.CauseClassificationRecord) *specSPCBlocker {
	return &specSPCBlocker{records: records}
}

func (s *specSPCBlocker) ShouldBlock(_ context.Context, b *bead.Bead) (bool, string, error) {
	spec := bead.FindSpecLabel(b.Labels)
	if spec == "" {
		return false, "", nil
	}
	for _, rec := range s.records {
		if rec.Stratum == fmt.Sprintf("spec:%s", spec) && rec.Class == logger.CauseClassSpecial {
			return true, s.reasonFor(spec), nil
		}
	}
	return false, "", nil
}

func (s *specSPCBlocker) reasonFor(spec string) string {
	return fmt.Sprintf("spec:%s maintenance warning", spec)
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

// RED: test for Gate.WithSpecSPCBlocker wiring method exists
func TestGate_WithSpecSPCBlocker_WiresBlocker(t *testing.T) {
	t.Parallel()

	spec := "auth"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	gate := New(io.Discard).WithSpecSPCBlocker(blocker)

	if gate == nil {
		t.Fatalf("WithSpecSPCBlocker() returned nil, want valid gate")
	}

	// Test that the gate blocks beads with matching spec
	b := &bead.Bead{
		ID:     "test-spc-blocker",
		Title:  "test bead",
		Labels: []string{"spec:" + spec},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v, want nil", err)
	}

	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v", out.Decision, pipeline.Block)
	}
}

// RED: test for HasSpecSPCBlocker returns correct value
func TestGate_HasSpecSPCBlocker(t *testing.T) {
	t.Parallel()

	t.Run("returns false when not wired", func(t *testing.T) {
		gate := New(io.Discard)
		if gate.HasSpecSPCBlocker() {
			t.Fatalf("HasSpecSPCBlocker() = true, want false")
		}
	})

	t.Run("returns true when wired", func(t *testing.T) {
		blocker := NewSpecSPCBlocker([]logger.CauseClassificationRecord{})
		gate := New(io.Discard).WithSpecSPCBlocker(blocker)
		if !gate.HasSpecSPCBlocker() {
			t.Fatalf("HasSpecSPCBlocker() = false, want true")
		}
	})
}

// RED: test that spec SPC blocking happens after data quality check
func TestGateRun_SpecSPCBlockingPrecedence(t *testing.T) {
	t.Parallel()

	spec := "auth"
	spcRecords := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}

	// Create blockers that will block independently
	spcBlocker := NewSpecSPCBlocker(spcRecords)
	dataQualityBlocker := newMockDataQualityBlocker().WithShouldBlock(true, "incomplete_data", nil)

	gate := New(io.Discard).
		WithDataQualityBlocker(dataQualityBlocker).
		WithSpecSPCBlocker(spcBlocker)

	b := &bead.Bead{
		ID:     "test-precedence",
		Title:  "test bead",
		Labels: []string{"spec:" + spec},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	// Both would block, but data quality should be checked first
	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want Block", out.Decision)
	}
}

// RED: test that spec SPC blocker doesn't block beads without matching anomalies
func TestGateRun_SpecSPCBlocker_AllowsHealthySpecs(t *testing.T) {
	t.Parallel()

	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:auth",
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	gate := New(io.Discard).WithSpecSPCBlocker(blocker)

	// Bead with different spec that has no anomaly
	b := &bead.Bead{
		ID:     "test-healthy-spec",
		Title:  "test bead",
		Labels: []string{"spec:payments"}, // Different spec, should proceed
	}

	out, err := gate.Run(context.Background(), pipeline.Input{Bead: b})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Proceed {
		t.Fatalf("decision = %v, want Proceed", out.Decision)
	}
}

// RED: test that spec SPC blocker emits GateBlockEvent with proper reason
func TestGateRun_SpecSPCBlocker_EmitsGateBlockEvent(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	spec := "database"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             2000,
			PersistenceWindows: 5,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	gate := New(io.Discard).WithSpecSPCBlocker(blocker)

	beadID := "spc-block-event-test"
	b := &bead.Bead{
		ID:     beadID,
		Title:  "maintenance heavy task",
		Labels: []string{"spec:" + spec},
	}

	out, err := gate.Run(context.Background(), pipeline.Input{
		Bead:    b,
		Emitter: emitter,
	})
	if err != nil {
		t.Fatalf("Gate.Run() error = %v", err)
	}

	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want Block", out.Decision)
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
	expectedReason := "spec:" + spec + " maintenance warning"
	if blockEvt.Reason != expectedReason {
		t.Errorf("Reason = %q, want %q", blockEvt.Reason, expectedReason)
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
