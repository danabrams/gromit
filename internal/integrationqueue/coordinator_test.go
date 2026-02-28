package integrationqueue

import (
	"context"
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

func findEntry(entries []Entry, branch string) *Entry {
	for i := range entries {
		if entries[i].Branch == branch {
			return &entries[i]
		}
	}
	return nil
}
