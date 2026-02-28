package integrationqueue

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCoordinatorRetriesGatesAfterInitialFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/retry",
		SessionID:        "feature/retry",
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

	gitops := &mockGitOps{}
	gate := &countingScopedGate{failures: 1}
	coord := NewCoordinator(store, gitops, gate)

	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v", err)
	}

	if gate.runCount != 2 {
		t.Fatalf("gate run count = %d, want 2", gate.runCount)
	}

	fetchCalls := countPrefixCalls(gitops.calls, "fetch:")
	if fetchCalls != 2 {
		t.Fatalf("fetch count = %d, want 2", fetchCalls)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, entry.Branch)
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StateMerged {
		t.Fatalf("State = %q, want %q", processed.State, StateMerged)
	}
	if processed.AttemptCount != 1 {
		t.Fatalf("AttemptCount = %d, want 1", processed.AttemptCount)
	}
	if processed.RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", processed.RetryCount)
	}
}

type countingScopedGate struct {
	failures int
	runCount int
}

func (g *countingScopedGate) Run(ctx context.Context, entry Entry) error {
	g.runCount++
	if g.runCount <= g.failures {
		return fmt.Errorf("gate failure %d", g.runCount)
	}
	return nil
}

func countPrefixCalls(calls []string, prefix string) int {
	count := 0
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			count++
		}
	}
	return count
}

func TestCoordinatorFailsAfterExhaustingGateRetries(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/fail",
		SessionID:        "feature/fail",
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

	gitops := &mockGitOps{}
	gate := &countingScopedGate{failures: 2}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate returns nil for terminal failure (StateFailedGates)
	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, entry.Branch)
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StateFailedGates {
		t.Fatalf("State = %q, want %q", processed.State, StateFailedGates)
	}
	if processed.RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", processed.RetryCount)
	}
	if gate.runCount != 2 {
		t.Fatalf("gate run count = %d, want 2", gate.runCount)
	}
	fetchCalls := countPrefixCalls(gitops.calls, "fetch:")
	if fetchCalls != 2 {
		t.Fatalf("fetch count = %d, want 2", fetchCalls)
	}
}

func TestCoordinatorGateRetryRecordsSingleRetry(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry := Entry{
		Branch:           "feature/single-retry",
		SessionID:        "feature/single-retry",
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

	gitops := &mockGitOps{}
	gate := &countingScopedGate{failures: 2}
	coord := NewCoordinator(store, gitops, gate)

	// Coordinate returns nil for terminal failure (StateFailedGates)
	if err := coord.Coordinate(ctx); err != nil {
		t.Fatalf("Coordinate() error = %v, want nil", err)
	}

	payload, err := store.load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	processed := findEntry(payload.Entries, entry.Branch)
	if processed == nil {
		t.Fatalf("missing processed entry")
	}
	if processed.State != StateFailedGates {
		t.Fatalf("State = %q, want %q", processed.State, StateFailedGates)
	}
	if processed.RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", processed.RetryCount)
	}
	if gate.runCount != 2 {
		t.Fatalf("gate run count = %d, want 2", gate.runCount)
	}
	fetchCalls := countPrefixCalls(gitops.calls, "fetch:")
	if fetchCalls != 2 {
		t.Fatalf("fetch count = %d, want 2", fetchCalls)
	}
}
