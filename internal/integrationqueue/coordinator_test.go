package integrationqueue

import (
	"context"
	"fmt"
	"reflect"
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

type selectiveConflictMockGitOps struct {
	calls []string
}

func (m *selectiveConflictMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "fetch:"+entry.Branch)
	return nil
}

func (m *selectiveConflictMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "merge:"+entry.Branch)
	// First entry conflicts, second succeeds
	if entry.Branch == "feature/will-conflict" {
		return fmt.Errorf("merge conflict")
	}
	return nil
}

func (m *selectiveConflictMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return nil
}

func (m *selectiveConflictMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "cleanup:"+entry.Branch)
	return nil
}

type selectiveFailGateMockGate struct {
	calls []string
}

func (m *selectiveFailGateMockGate) Run(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, entry.Branch)
	// First entry fails gates, second succeeds
	if entry.Branch == "feature/will-fail-gates" {
		return fmt.Errorf("gate validation failed")
	}
	return nil
}

type selectiveLaneViolationMockGitOps struct {
	calls []string
}

func (m *selectiveLaneViolationMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "fetch:"+entry.Branch)
	return nil
}

func (m *selectiveLaneViolationMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "merge:"+entry.Branch)
	// First entry has lane violation, second succeeds
	if entry.Branch == "feature/will-lane-violate" {
		return fmt.Errorf("lane violation: cannot merge safe_lane with code changes")
	}
	return nil
}

func (m *selectiveLaneViolationMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return nil
}

func (m *selectiveLaneViolationMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "cleanup:"+entry.Branch)
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

	// Coordinate should return nil for terminal failure (no blocking)
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
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

	// Coordinate should return nil for terminal failure (no blocking)
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
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

// TestCoordinator_PreservesBranchStateOnMultipleTerminalFailures verifies that
// when multiple ready entries experience terminal failures, each branch is
// properly transitioned to its correct terminal state without corruption.
func TestCoordinator_PreservesBranchStateOnMultipleTerminalFailures(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Three entries with different terminal failures
	conflictEntry := Entry{
		Branch:           "feature/conflict",
		SessionID:        "session-conflict",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(conflictEntry); err != nil {
		t.Fatalf("Save(conflictEntry) error = %v", err)
	}

	laneViolationEntry := Entry{
		Branch:           "feature/lane-violation",
		SessionID:        "session-lane-viol",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "safe_lane",
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(laneViolationEntry); err != nil {
		t.Fatalf("Save(laneViolationEntry) error = %v", err)
	}

	failGatesEntry := Entry{
		Branch:           "feature/fail-gates",
		SessionID:        "session-fail-gates",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "beefdead",
		ChangedFilesHash: "hash3",
	}
	if err := store.Save(failGatesEntry); err != nil {
		t.Fatalf("Save(failGatesEntry) error = %v", err)
	}

	gitops := &multiTerminalMockGitOps{}
	gate := &multiTerminalMockGate{}
	coord := NewCoordinator(store, gitops, gate)

	// All three should be processed in one loop
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	// Verify each entry is in correct terminal state
	conflict := findEntry(payload.Entries, "feature/conflict")
	if conflict == nil {
		t.Fatal("conflict entry not found")
	}
	if conflict.State != StateConflict {
		t.Fatalf("conflict.State = %q, want %q", conflict.State, StateConflict)
	}
	if conflict.LastErrorCode != "merge_conflict" {
		t.Fatalf("conflict.LastErrorCode = %q, want merge_conflict", conflict.LastErrorCode)
	}

	laneViol := findEntry(payload.Entries, "feature/lane-violation")
	if laneViol == nil {
		t.Fatal("lane violation entry not found")
	}
	if laneViol.State != StateLaneViolation {
		t.Fatalf("laneViol.State = %q, want %q", laneViol.State, StateLaneViolation)
	}
	if laneViol.LastErrorCode != "lane_violation" {
		t.Fatalf("laneViol.LastErrorCode = %q, want lane_violation", laneViol.LastErrorCode)
	}

	failGates := findEntry(payload.Entries, "feature/fail-gates")
	if failGates == nil {
		t.Fatal("fail gates entry not found")
	}
	if failGates.State != StateFailedGates {
		t.Fatalf("failGates.State = %q, want %q", failGates.State, StateFailedGates)
	}
	if failGates.LastErrorCode != "failed_gates" {
		t.Fatalf("failGates.LastErrorCode = %q, want failed_gates", failGates.LastErrorCode)
	}
}

type multiTerminalMockGitOps struct{}

func (m *multiTerminalMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	return nil
}

func (m *multiTerminalMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	if entry.Branch == "feature/conflict" {
		return fmt.Errorf("merge conflict")
	}
	if entry.Branch == "feature/lane-violation" {
		return fmt.Errorf("lane violation: cannot merge safe_lane with code changes")
	}
	return nil
}

func (m *multiTerminalMockGitOps) Push(ctx context.Context) error {
	return nil
}

func (m *multiTerminalMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	return nil
}

type multiTerminalMockGate struct{}

func (g *multiTerminalMockGate) Run(ctx context.Context, entry Entry) error {
	if entry.Branch == "feature/fail-gates" {
		return fmt.Errorf("gate validation failed")
	}
	return nil
}

// TestCoordinator_StallsOnNonTerminalFailure verifies that non-terminal failures
// (like push failures) still block FIFO progression as they are not recoverable
// in the same loop iteration.
func TestCoordinator_StallsOnNonTerminalFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Entry that will fail during push
	pushFailEntry := Entry{
		Branch:           "feature/push-fail",
		SessionID:        "feature/push-fail",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(pushFailEntry); err != nil {
		t.Fatalf("Save(pushFailEntry) error = %v", err)
	}

	// Second entry should NOT be processed if first fails push
	neverProcessed := Entry{
		Branch:           "feature/never-process",
		SessionID:        "feature/never-process",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(neverProcessed); err != nil {
		t.Fatalf("Save(neverProcessed) error = %v", err)
	}

	gitops := &pushFailureMockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate should return error for non-terminal failure
	err = coord.Coordinate(ctx)
	if err == nil {
		t.Fatalf("Coordinate() error = nil, want non-nil")
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	// First entry should be in integrating or have failed_gates (before push failed)
	first := findEntry(payload.Entries, "feature/push-fail")
	if first == nil {
		t.Fatalf("missing first entry")
	}

	// Second entry should still be ready (not processed)
	second := findEntry(payload.Entries, "feature/never-process")
	if second == nil {
		t.Fatalf("missing second entry")
	}
	if second.State != StateReady {
		t.Fatalf("second.State = %q, want %q (should not have been processed)", second.State, StateReady)
	}
}

type pushFailureMockGitOps struct {
	calls []string
}

func (m *pushFailureMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "fetch:"+entry.Branch)
	return nil
}

func (m *pushFailureMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "merge:"+entry.Branch)
	return nil
}

func (m *pushFailureMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "push")
	return fmt.Errorf("push to remote failed: network timeout")
}

func (m *pushFailureMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "cleanup:"+entry.Branch)
	return nil
}

// TestCoordinator_DoesNotStallOnLaneViolation verifies that when entry A has lane violation,
// entry B (next ready) is processed in the same Coordinate call instead of blocking.
func TestCoordinator_DoesNotStallOnLaneViolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// First entry will have lane violation
	laneViolationEntry := Entry{
		Branch:           "feature/will-lane-violate",
		SessionID:        "feature/will-lane-violate",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "safe_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(laneViolationEntry); err != nil {
		t.Fatalf("Save(laneViolationEntry) error = %v", err)
	}

	// Second entry should be processed after first has lane violation
	succeedEntry := Entry{
		Branch:           "feature/will-succeed",
		SessionID:        "feature/will-succeed",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(succeedEntry); err != nil {
		t.Fatalf("Save(succeedEntry) error = %v", err)
	}

	gitops := &selectiveLaneViolationMockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate should process both entries (first has lane violation, second succeeds)
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	// First entry should be in lane_violation state
	first := findEntry(payload.Entries, "feature/will-lane-violate")
	if first == nil {
		t.Fatalf("missing first entry")
	}
	if first.State != StateLaneViolation {
		t.Fatalf("first.State = %q, want %q", first.State, StateLaneViolation)
	}

	// Second entry should be merged (successfully processed)
	second := findEntry(payload.Entries, "feature/will-succeed")
	if second == nil {
		t.Fatalf("missing second entry")
	}
	if second.State != StateMerged {
		t.Fatalf("second.State = %q, want %q", second.State, StateMerged)
	}
}

// TestCoordinator_DoesNotStallOnFailedGates verifies that when entry A fails gates,
// entry B (next ready) is processed in the same Coordinate call instead of blocking.
func TestCoordinator_DoesNotStallOnFailedGates(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// First entry will fail gates
	failGatesEntry := Entry{
		Branch:           "feature/will-fail-gates",
		SessionID:        "feature/will-fail-gates",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(failGatesEntry); err != nil {
		t.Fatalf("Save(failGatesEntry) error = %v", err)
	}

	// Second entry should be processed after first fails gates
	succeedEntry := Entry{
		Branch:           "feature/will-succeed",
		SessionID:        "feature/will-succeed",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(succeedEntry); err != nil {
		t.Fatalf("Save(succeedEntry) error = %v", err)
	}

	gitops := &mockGitOps{}
	gate := &selectiveFailGateMockGate{}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate should process both entries (first fails gates, second succeeds)
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	// First entry should be in failed_gates state
	first := findEntry(payload.Entries, "feature/will-fail-gates")
	if first == nil {
		t.Fatalf("missing first entry")
	}
	if first.State != StateFailedGates {
		t.Fatalf("first.State = %q, want %q", first.State, StateFailedGates)
	}

	// Second entry should be merged (successfully processed)
	second := findEntry(payload.Entries, "feature/will-succeed")
	if second == nil {
		t.Fatalf("missing second entry")
	}
	if second.State != StateMerged {
		t.Fatalf("second.State = %q, want %q", second.State, StateMerged)
	}
}

// TestCoordinator_DoesNotStallOnConflict verifies that when entry A conflicts,
// entry B (next ready) is processed in the same Coordinate call instead of blocking.
func TestCoordinator_DoesNotStallOnConflict(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// First entry will conflict
	conflictEntry := Entry{
		Branch:           "feature/will-conflict",
		SessionID:        "feature/will-conflict",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(conflictEntry); err != nil {
		t.Fatalf("Save(conflictEntry) error = %v", err)
	}

	// Second entry should be processed after first conflicts
	succeedEntry := Entry{
		Branch:           "feature/will-succeed",
		SessionID:        "feature/will-succeed",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(succeedEntry); err != nil {
		t.Fatalf("Save(succeedEntry) error = %v", err)
	}

	// GitOps that conflicts on first call, succeeds on second
	gitops := &selectiveConflictMockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate should process both entries (first conflicts, second succeeds)
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	// First entry should be in conflict state
	first := findEntry(payload.Entries, "feature/will-conflict")
	if first == nil {
		t.Fatalf("missing first entry")
	}
	if first.State != StateConflict {
		t.Fatalf("first.State = %q, want %q", first.State, StateConflict)
	}

	// Second entry should be merged (successfully processed)
	second := findEntry(payload.Entries, "feature/will-succeed")
	if second == nil {
		t.Fatalf("missing second entry")
	}
	if second.State != StateMerged {
		t.Fatalf("second.State = %q, want %q", second.State, StateMerged)
	}
}

// TestCoordinatorProcessNext verifies ProcessNext processes a single entry
// and returns true when an entry was processed, false when none are ready.
func TestCoordinatorProcessNext(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/test",
		SessionID:        "feature/test",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFiles:     []string{"test.txt"},
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	gitops := &mockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// ProcessNext should process the single entry
	processed, err := coord.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatalf("ProcessNext() = %v, want true", processed)
	}

	// Entry should be merged after ProcessNext
	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	found := findEntry(payload.Entries, "feature/test")
	if found == nil {
		t.Fatalf("entry not found")
	}
	if found.State != StateMerged {
		t.Fatalf("State = %q, want %q", found.State, StateMerged)
	}
}

// TestCoordinatorProcessNext_RunsSafetyValidation verifies that ProcessNext
// runs safety validation first and fails the entry on violation.
func TestCoordinatorProcessNext_RunsSafetyValidation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create entry with prohibited artifact (lock file)
	entry := Entry{
		Branch:           "feature/unsafe",
		SessionID:        "feature/unsafe",
		OriginCommand:    "test",
		State:            StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFiles:     []string{"go.sum"},  // Lock file - safety violation
		ChangedFilesHash: "hash",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	gitops := &mockGitOps{}
	gate := &mockScopedGate{}
	coord := NewCoordinator(store, gitops, gate)

	// ProcessNext should fail due to safety violation
	processed, err := coord.ProcessNext(ctx)
	if err == nil {
		t.Fatalf("ProcessNext() error = nil, want safety violation error")
	}
	if processed {
		t.Fatalf("ProcessNext() = %v, want false (safety violation)", processed)
	}

	// Entry should be transitioned to conflict state on safety violation
	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	found := findEntry(payload.Entries, "feature/unsafe")
	if found == nil {
		t.Fatalf("entry not found")
	}
	if found.State != StateConflict {
		t.Fatalf("State = %q, want %q (safety violations are terminal)", found.State, StateConflict)
	}
	if found.LastErrorCode == "" {
		t.Fatalf("LastErrorCode should be set for safety violation")
	}

	// GitOps should NOT have been called (safety validation runs first)
	if len(gitops.calls) != 0 {
		t.Fatalf("gitops.calls = %v, want [] (safety check should prevent gitops calls)", gitops.calls)
	}
}
