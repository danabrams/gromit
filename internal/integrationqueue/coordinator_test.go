package integrationqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCoordinatorProcessesOldestReadyEntry(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	older := Entry{
		Branch:           "feature/old",
		SessionID:        "feature/old",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFiles:     []string{"old.txt"},
		ChangedFilesHash: "hash",
	}
	if err := store.Save(older); err != nil {
		t.Fatalf("Save(older) error = %v", err)
	}

	newer := older
	newer.Branch = "feature/new"
	newer.SessionID = "feature/new"
	if err := store.Save(newer); err != nil {
		t.Fatalf("Save(newer) error = %v", err)
	}

	gitops := &mockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	processed := findEntry(payload.Entries, "feature/old")
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StateMerged {
		t.Fatalf("processed.State = %q, want %q", processed.State, StateMerged)
	}
	remaining := findEntry(payload.Entries, "feature/new")
	if remaining == nil {
		t.Fatalf("missing remaining entry")
	}
	if remaining.State != StateReady {
		t.Fatalf("remaining.State = %q, want %q", remaining.State, StateReady)
	}

	wantCalls := []string{
		"fetch:feature/old",
		"merge:feature/old",
		"push",
		"cleanup:feature/old",
	}
	if !reflect.DeepEqual(gitops.calls, wantCalls) {
		t.Fatalf("gitops.calls = %v, want %v", gitops.calls, wantCalls)
	}

	if len(gate.calls) != 1 || gate.calls[0] != "feature/old" {
		t.Fatalf("gate.calls = %v, want [\"feature/old\"]", gate.calls)
	}
}

func TestCoordinatorReportsTransitionErrorOnLaneViolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/lane-violation",
		SessionID:        "feature/lane-violation",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	stateTransitions := allowedTransitions[string(StateIntegrating)]
	original := stateTransitions[string(StateLaneViolation)]
	stateTransitions[string(StateLaneViolation)] = false
	t.Cleanup(func() {
		stateTransitions[string(StateLaneViolation)] = original
	})

	coord := NewCoordinator(store, &laneViolationMockGitOps{}, &mockScopedGate{})

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want transition failure")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Coordinate() error = %v, want ErrInvalidTransition", err)
	}
}

func TestCoordinatorPropagatesSaveErrorAfterInitialFetchFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	saveErr := errors.New("initial transition save failed")

	store, err := NewStore(tmpDir, WithValidationHook(func(entry Entry) error {
		if entry.Branch == "feature/save-fail-initial" && entry.State == StateConflict {
			return saveErr
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.Save(Entry{
		Branch:           "feature/save-fail-initial",
		SessionID:        "feature/save-fail-initial",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &failFetchGitOps{err: errors.New("fetch failed")}
	coord := NewCoordinator(store, gitops, &mockScopedGate{})

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want store save error")
	}
	if !errors.Is(err, saveErr) {
		t.Fatalf("Coordinate() error = %v, want %v", err, saveErr)
	}
}

func TestCoordinatorPropagatesSaveErrorAfterGateRetryFetchFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	saveErr := errors.New("retry transition save failed")

	store, err := NewStore(tmpDir, WithValidationHook(func(entry Entry) error {
		if entry.Branch == "feature/save-fail-retry" && entry.State == StateConflict {
			return saveErr
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/save-fail-retry",
		SessionID:        "feature/save-fail-retry",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	retryErr := errors.New("retry fetch failed")
	gitops := &retryFetchFailGitOps{failErr: retryErr}
	gate := &failGateOnce{}
	coord := NewCoordinator(store, gitops, gate)

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want store save error")
	}
	if !errors.Is(err, saveErr) {
		t.Fatalf("Coordinate() error = %v, want %v", err, saveErr)
	}
}

func TestCoordinatorPropagatesTransitionErrorOnFetchConflict(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	stateTransitions := allowedTransitions[string(StateIntegrating)]
	originalConflict := stateTransitions[string(StateConflict)]
	stateTransitions[string(StateConflict)] = false
	t.Cleanup(func() {
		stateTransitions[string(StateConflict)] = originalConflict
	})

	entry := Entry{
		Branch:           "feature/transition-error",
		SessionID:        "feature/transition-error",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &failFetchGitOps{err: errors.New("fetch failed")}
	coord := NewCoordinator(store, gitops, &mockScopedGate{})

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want ErrInvalidTransition")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Coordinate() error = %v, want ErrInvalidTransition", err)
	}
}

func TestCoordinatorClassifiesCheckoutFailureDistinctly(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/checkout-failure",
		SessionID:        "feature/checkout-failure",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &failFetchGitOps{err: errors.New("checkout branch feature/checkout-failure: exit status 1")}
	coord := NewCoordinator(store, gitops, &mockScopedGate{})

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want checkout failure")
	}

	payload, loadErr := store.load()
	if loadErr != nil {
		t.Fatalf("load() error = %v", loadErr)
	}
	processed := findEntry(payload.Entries, entry.Branch)
	if processed == nil {
		t.Fatalf("missing processed entry %q", entry.Branch)
	}
	if processed.State != StateConflict {
		t.Fatalf("processed.State = %q, want %q", processed.State, StateConflict)
	}
	if processed.LastErrorCode != "checkout_failed" {
		t.Fatalf("LastErrorCode = %q, want checkout_failed", processed.LastErrorCode)
	}
	if processed.LastTransitionReason != "checkout failed during initial fetch" {
		t.Fatalf("LastTransitionReason = %q, want checkout failed during initial fetch", processed.LastTransitionReason)
	}
}

type mockGitOps struct {
	calls []string
}

func (m *mockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "fetch:"+entry.Branch)
	return nil
}

func (m *mockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "merge:"+entry.Branch)
	return nil
}

func (m *mockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return nil
}

func (m *mockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "cleanup:"+entry.Branch)
	return nil
}

type mockScopedGate struct {
	calls []string
}

func (m *mockScopedGate) Run(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, entry.Branch)
	return nil
}

type conflictMockGitOps struct {
	calls []string
}

func (m *conflictMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "fetch:"+entry.Branch)
	return nil
}

func (m *conflictMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "merge:"+entry.Branch)
	return fmt.Errorf("merge conflict")
}

func (m *conflictMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return nil
}

func (m *conflictMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "cleanup:"+entry.Branch)
	return nil
}

type laneViolationMockGitOps struct {
	calls []string
}

func (m *laneViolationMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "fetch:"+entry.Branch)
	return nil
}

func (m *laneViolationMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "merge:"+entry.Branch)
	return fmt.Errorf("lane violation: cannot merge safe_lane with code changes")
}

func (m *laneViolationMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return nil
}

func (m *laneViolationMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "cleanup:"+entry.Branch)
	return nil
}

type failFetchGitOps struct {
	err error
}

func (m *failFetchGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *failFetchGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	return nil
}

func (m *failFetchGitOps) Push(ctx context.Context) error {
	return nil
}

func (m *failFetchGitOps) Cleanup(ctx context.Context, entry Entry) error {
	return nil
}

type retryFetchFailGitOps struct {
	failErr    error
	fetchCount int
}

func (m *retryFetchFailGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.fetchCount++
	if m.fetchCount == 2 {
		return m.failErr
	}
	return nil
}

func (m *retryFetchFailGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	return nil
}

func (m *retryFetchFailGitOps) Push(ctx context.Context) error {
	return nil
}

func (m *retryFetchFailGitOps) Cleanup(ctx context.Context, entry Entry) error {
	return nil
}

type failGateOnce struct {
	callCount int
}

func (m *failGateOnce) Run(ctx context.Context, entry Entry) error {
	m.callCount++
	if m.callCount == 1 {
		return errors.New("scoped gate failed")
	}
	return nil
}

func TestCoordinatorIncrementsAttemptCount(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:               "feature/old",
		SessionID:            "feature/old",
		OriginCommand:        "test",
		State:                StateReady,
		Lane:                 "code_lane",
		BaseRef:              "main",
		HeadSHA:              "deadbeef",
		ChangedFiles:         []string{"old.txt"},
		ChangedFilesHash:     "hash",
		AttemptCount:         0,
		RetryCount:           0,
		LastErrorCode:        "code",
		LastErrorMessage:     "message",
		LastTransitionReason: "entered queue",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	copyEntry := entry
	copyEntry.Branch = "feature/new"
	copyEntry.SessionID = "feature/new"
	if err := store.Save(copyEntry); err != nil {
		t.Fatalf("Save(copyEntry) error = %v", err)
	}

	gitops := &mockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/old")
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.AttemptCount != 1 {
		t.Fatalf("processed.AttemptCount = %d, want 1", processed.AttemptCount)
	}

	remaining := findEntry(payload.Entries, "feature/new")
	if remaining == nil {
		t.Fatalf("missing remaining entry")
	}
	if remaining.AttemptCount != 0 {
		t.Fatalf("remaining.AttemptCount = %d, want 0", remaining.AttemptCount)
	}
}

func findEntry(entries []Entry, branch string) *Entry {
	for i := range entries {
		if entries[i].Branch == branch {
			return &entries[i]
		}
	}
	return nil
}

// TestCoordinator_RecoverFromCrash verifies that RecoverFromCrash() detects
// entries left in StateIntegrating and transitions them back to StateReady.
// This ensures that if gromit crashes mid-integration, entries aren't left
// stranded in the integrating state.
func TestCoordinator_RecoverFromCrash(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create entries in various states, including some in integrating
	readyEntry := Entry{
		Branch:           "feature/ready",
		SessionID:        "session1",
		OriginCommand:    "refine",
		State:            StateReady,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(readyEntry); err != nil {
		t.Fatalf("Save(readyEntry) error = %v", err)
	}

	integratingEntry1 := Entry{
		Branch:           "feature/integrating1",
		SessionID:        "session2",
		OriginCommand:    "refine",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(integratingEntry1); err != nil {
		t.Fatalf("Save(integratingEntry1) error = %v", err)
	}

	integratingEntry2 := Entry{
		Branch:           "feature/integrating2",
		SessionID:        "session3",
		OriginCommand:    "refine",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "beefdead",
		ChangedFilesHash: "hash3",
	}
	if err := store.Save(integratingEntry2); err != nil {
		t.Fatalf("Save(integratingEntry2) error = %v", err)
	}

	coord := NewCoordinator(store, &mockGitOps{}, &mockScopedGate{})

	// Recover from crash
	err = coord.RecoverFromCrash(ctx)
	if err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}

	// Verify recovery results
	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	if len(payload.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(payload.Entries))
	}

	// Ready entry should remain unchanged
	ready := findEntry(payload.Entries, "feature/ready")
	if ready == nil {
		t.Fatal("ready entry not found")
	}
	if ready.State != StateReady {
		t.Fatalf("ready entry state = %s, want %s", ready.State, StateReady)
	}

	// Integrating entries should be reset to ready
	integ1 := findEntry(payload.Entries, "feature/integrating1")
	if integ1 == nil {
		t.Fatal("integrating1 entry not found")
	}
	if integ1.State != StateReady {
		t.Fatalf("integrating1 state = %s, want %s", integ1.State, StateReady)
	}
	if integ1.LastErrorCode != "crash_recovery" {
		t.Fatalf("integrating1 error code = %s, want crash_recovery", integ1.LastErrorCode)
	}

	integ2 := findEntry(payload.Entries, "feature/integrating2")
	if integ2 == nil {
		t.Fatal("integrating2 entry not found")
	}
	if integ2.State != StateReady {
		t.Fatalf("integrating2 state = %s, want %s", integ2.State, StateReady)
	}
	if integ2.LastErrorCode != "crash_recovery" {
		t.Fatalf("integrating2 error code = %s, want crash_recovery", integ2.LastErrorCode)
	}
}

func TestCoordinatorRecoverFromCrash_PartialErrorMetadata(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	queuePath := filepath.Join(tmpDir, queueFileName)
	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries: []Entry{
			{
				Branch:           "feature/partial-error",
				SessionID:        "session-partial",
				OriginCommand:    "test",
				State:            StateIntegrating,
				Lane:             string(CodeLane),
				BaseRef:          "main",
				HeadSHA:          "deadbeef",
				ChangedFilesHash: "hash",
				LastErrorMessage: "fallback failure",
			},
		},
	}
	data, err := json.Marshal(queue)
	if err != nil {
		t.Fatalf("json.Marshal(queue) error = %v", err)
	}
	if err := os.WriteFile(queuePath, data, 0o644); err != nil {
		t.Fatalf("write queue file: %v", err)
	}

	coord := &Coordinator{store: store}

	if err := coord.RecoverFromCrash(ctx); err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/partial-error")
	if processed == nil {
		t.Fatal("missing processed entry")
	}
	if processed.State != StateReady {
		t.Fatalf("State = %q, want %q", processed.State, StateReady)
	}
	if processed.LastErrorCode != string(CrashRecoveryErrorCode) {
		t.Fatalf("LastErrorCode = %q, want %q", processed.LastErrorCode, string(CrashRecoveryErrorCode))
	}
	if processed.LastErrorMessage != crashRecoveryMessage {
		t.Fatalf("LastErrorMessage = %q", processed.LastErrorMessage)
	}
}

func TestCoordinatorRecoverFromCrash_RecordsErrorMetadata(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/crash-metadata",
		SessionID:        "feature/crash-metadata",
		OriginCommand:    "test",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	coord := &Coordinator{store: store}

	var recorded TransitionErrorMetadata
	recordedCount := 0
	coord.transitionFn = func(entry *Entry, toState string, reason string, metadata ...TransitionErrorMetadata) error {
		if toState == string(StateReady) && strings.Contains(reason, "crash recovery") {
			recordedCount++
			if len(metadata) > 0 {
				recorded = metadata[0]
			}
		}
		return ApplyTransition(entry, toState, reason, metadata...)
	}

	if err := coord.RecoverFromCrash(ctx); err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}
	if recordedCount != 1 {
		t.Fatalf("expected metadata recorded once, got %d", recordedCount)
	}
	want := crashRecoveryMetadata()
	if recorded != want {
		t.Fatalf("metadata = %+v, want %+v", recorded, want)
	}
}

func TestCoordinatorRecoverFromCrash_FailedGates(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	entry := Entry{
		Branch:           "feature/failed-gates",
		SessionID:        "feature/failed-gates",
		OriginCommand:    "test",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		FifoSeq:          1,
		LastErrorCode:    string(StateFailedGates),
		LastErrorMessage: "scoped gates failed",
	}
	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries:       []Entry{entry},
	}
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		t.Fatalf("marshal queue: %v", err)
	}
	if err := os.WriteFile(queuePath, data, 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	coord := &Coordinator{store: store}

	if err := coord.RecoverFromCrash(ctx); err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/failed-gates")
	if processed == nil {
		t.Fatal("missing processed entry")
	}
	if processed.State != StateFailedGates {
		t.Fatalf("State = %q, want %q", processed.State, StateFailedGates)
	}
	if processed.LastErrorCode != string(StateFailedGates) {
		t.Fatalf("LastErrorCode = %q, want %q", processed.LastErrorCode, string(StateFailedGates))
	}
}

func TestCoordinatorRecoverFromCrash_PassesErrorMetadataDuringTransition(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	entry := Entry{
		Branch:           "feature/failed-gates-transition",
		SessionID:        "feature/failed-gates-transition",
		OriginCommand:    "test",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		FifoSeq:          1,
		LastErrorCode:    string(StateFailedGates),
		LastErrorMessage: "scoped gates failed",
	}
	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries:       []Entry{entry},
	}
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		t.Fatalf("marshal queue: %v", err)
	}
	if err := os.WriteFile(queuePath, data, 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	coord := &Coordinator{store: store}

	metadataSeen := false
	var recorded TransitionErrorMetadata
	coord.transitionFn = func(entry *Entry, toState string, reason string, metadata ...TransitionErrorMetadata) error {
		if toState == string(StateFailedGates) && strings.Contains(reason, "failed gates") {
			metadataSeen = metadataSeen || len(metadata) > 0
			if len(metadata) > 0 {
				recorded = metadata[0]
			}
		}
		return ApplyTransition(entry, toState, reason, metadata...)
	}

	if err := coord.RecoverFromCrash(ctx); err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}

	if !metadataSeen {
		t.Fatal("expected metadata to be passed to ApplyTransition during crash recovery")
	}
	if recorded.Code != string(StateFailedGates) {
		t.Fatalf("metadata.Code = %q, want %q", recorded.Code, string(StateFailedGates))
	}
	if recorded.Message != "scoped gates failed" {
		t.Fatalf("metadata.Message = %q, want %q", recorded.Message, "scoped gates failed")
	}
}

func TestCoordinatorRecoverFromCrash_MergeConflict(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	entry := Entry{
		Branch:           "feature/merge-conflict",
		SessionID:        "feature/merge-conflict",
		OriginCommand:    "test",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		FifoSeq:          1,
		LastErrorCode:    "merge_conflict",
		LastErrorMessage: "merge conflict detected",
	}
	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries:       []Entry{entry},
	}
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		t.Fatalf("marshal queue: %v", err)
	}
	if err := os.WriteFile(queuePath, data, 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	coord := &Coordinator{store: store}

	if err := coord.RecoverFromCrash(ctx); err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/merge-conflict")
	if processed == nil {
		t.Fatal("missing processed entry")
	}
	if processed.State != StateConflict {
		t.Fatalf("State = %q, want %q", processed.State, StateConflict)
	}
	if processed.LastErrorCode != "merge_conflict" {
		t.Fatalf("LastErrorCode = %q, want merge_conflict", processed.LastErrorCode)
	}
}

func TestCoordinatorRecoverFromCrash_PushFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	entry := Entry{
		Branch:           "feature/push-failure",
		SessionID:        "feature/push-failure",
		OriginCommand:    "test",
		State:            StateIntegrating,
		Lane:             string(CodeLane),
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		FifoSeq:          1,
		LastErrorCode:    "push_failed",
		LastErrorMessage: "push failure: remote rejected",
	}
	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries:       []Entry{entry},
	}
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		t.Fatalf("marshal queue: %v", err)
	}
	if err := os.WriteFile(queuePath, data, 0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	coord := &Coordinator{store: store}

	if err := coord.RecoverFromCrash(ctx); err != nil {
		t.Fatalf("RecoverFromCrash() error = %v", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/push-failure")
	if processed == nil {
		t.Fatal("missing processed entry")
	}
	if processed.State != StatePushFailure {
		t.Fatalf("State = %q, want %q", processed.State, StatePushFailure)
	}
	if processed.LastErrorCode != "push_failed" {
		t.Fatalf("LastErrorCode = %q, want push_failed", processed.LastErrorCode)
	}
}

func TestCoordinatorHandlesMergeConflict(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/conflict",
		SessionID:        "feature/conflict",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &conflictMockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate should return an error when merge fails
	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want non-nil")
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/conflict")
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StateConflict {
		t.Fatalf("State = %q, want %q", processed.State, StateConflict)
	}
	if processed.LastErrorCode != "merge_conflict" {
		t.Fatalf("LastErrorCode = %q, want merge_conflict", processed.LastErrorCode)
	}
}

func TestCoordinatorHandlesLaneViolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/lane-violation",
		SessionID:        "feature/lane-violation",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "safe_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &laneViolationMockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate should return an error when lane violation is detected
	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want non-nil")
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/lane-violation")
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StateLaneViolation {
		t.Fatalf("State = %q, want %q", processed.State, StateLaneViolation)
	}
	if processed.LastErrorCode != "lane_violation" {
		t.Fatalf("LastErrorCode = %q, want lane_violation", processed.LastErrorCode)
	}
}

func TestCoordinatorHandlesPushFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/push-failure",
		SessionID:        "feature/push-failure",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &pushFailureMockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want non-nil")
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, "feature/push-failure")
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StatePushFailure {
		t.Fatalf("State = %q, want %q", processed.State, StatePushFailure)
	}
	if processed.LastErrorCode != "push_failed" {
		t.Fatalf("LastErrorCode = %q, want push_failed", processed.LastErrorCode)
	}
	if !strings.Contains(processed.LastErrorMessage, "push failure") {
		t.Fatalf("LastErrorMessage = %q, want to contain 'push failure'", processed.LastErrorMessage)
	}
}

func TestCoordinatorPassesErrorMetadataThroughApplyTransition(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/error-meta",
		SessionID:        "feature/error-meta",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save(entry) error = %v", err)
	}

	gitops := &failFetchGitOps{err: errors.New("fetch failed")}
	coord := NewCoordinator(store, gitops, &mockScopedGate{})

	var conflictMetadata TransitionErrorMetadata
	recorded := false
	coord.transitionFn = func(entry *Entry, toState string, reason string, metadata ...TransitionErrorMetadata) error {
		if toState == string(StateConflict) && len(metadata) > 0 {
			conflictMetadata = metadata[0]
			recorded = true
		}
		return ApplyTransition(entry, toState, reason, metadata...)
	}

	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want non-nil")
	}
	if !recorded {
		t.Fatalf("expected metadata when transitioning to conflict")
	}
	if conflictMetadata.Code != "rebase_conflict" {
		t.Fatalf("metadata.Code = %q, want rebase_conflict", conflictMetadata.Code)
	}
	if conflictMetadata.Message != "fetch failed" {
		t.Fatalf("metadata.Message = %q, want fetch failed", conflictMetadata.Message)
	}
}

type pushFailureMockGitOps struct {
	mockGitOps
}

func (m *pushFailureMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return fmt.Errorf("push failure: remote rejected")
}
