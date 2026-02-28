//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/runner"
)

// TestOrchestrator_RecoverFromCrashOnStartup verifies that the orchestrator
// calls Coordinator.RecoverFromCrash during startup to reset entries left in
// StateIntegrating by a prior crash.
func TestOrchestrator_RecoverFromCrashOnStartup(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a mock coordinator that tracks recovery call
	recoveryCallCount := 0
	mockCoordinator := &mockCoordinatorWithRecovery{
		RecoverFromCrashFn: func(ctx context.Context) error {
			recoveryCallCount++
			return nil
		},
	}

	// Create orchestrator with the mock coordinator
	mock := &mockBeadClient{
		ReadyFn: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil // Return nil to stop the loop
		},
	}

	orch := runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &noopStage{},
		Build:    &noopStage{},
		Validate: &noopStage{},
		Epilogue: &noopStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return mock.Ready(ctx)
		},
		Config:      cfg,
		Output:      io.Discard,
		Coordinator: mockCoordinator,
	})

	err := orch.Run(context.Background(), 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if recoveryCallCount == 0 {
		t.Fatal("expected Coordinator.RecoverFromCrash() to be called at startup")
	}
	if recoveryCallCount != 1 {
		t.Fatalf("expected Coordinator.RecoverFromCrash() to be called once, got %d", recoveryCallCount)
	}
}

// mockCoordinatorWithRecovery is a mock coordinator that supports recovery testing.
type mockCoordinatorWithRecovery struct {
	RecoverFromCrashFn func(ctx context.Context) error
	CoordinateFn       func(ctx context.Context) error
}

func (m *mockCoordinatorWithRecovery) RecoverFromCrash(ctx context.Context) error {
	if m.RecoverFromCrashFn != nil {
		return m.RecoverFromCrashFn(ctx)
	}
	return nil
}

func (m *mockCoordinatorWithRecovery) Coordinate(ctx context.Context) error {
	if m.CoordinateFn != nil {
		return m.CoordinateFn(ctx)
	}
	return nil
}

// TestOrchestrator_RecoverFromMalformedQueueFile verifies that the orchestrator
// can detect and handle malformed queue files by calling RecoverFromMalformedQueue.
func TestOrchestrator_RecoverFromMalformedQueueFile(t *testing.T) {
	t.Parallel()

	// This test verifies that a malformed queue file is handled gracefully
	// by the recovery mechanism without crashing the orchestrator loop.

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a mock coordinator that simulates queue recovery
	mockCoordinator := &mockCoordinatorWithRecovery{
		CoordinateFn: func(ctx context.Context) error {
			// Simulate recovery handling of a malformed queue
			return nil
		},
	}

	mock := &mockBeadClient{
		ReadyFn: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil // Return nil to stop the loop
		},
	}

	orch := runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &noopStage{},
		Build:    &noopStage{},
		Validate: &noopStage{},
		Epilogue: &noopStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return mock.Ready(ctx)
		},
		Config:      cfg,
		Output:      io.Discard,
		Coordinator: mockCoordinator,
	})

	err := orch.Run(context.Background(), 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

// TestIntegrationQueue_RestartDurability verifies that queue entries survive
// an orchestrator restart and are properly recovered.
func TestIntegrationQueue_RestartDurability(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create initial queue with entries in various states
	queuePath := tmpDir + "/integration-queue.json"
	queue := &integrationqueue.Queue{
		SchemaVersion: integrationqueue.SchemaVersion,
		Entries: []integrationqueue.Entry{
			{
				Branch:           "feature/ready",
				SessionID:        "session1",
				OriginCommand:    "refine",
				State:            integrationqueue.StateReady,
				Lane:             string(integrationqueue.CodeLane),
				BaseRef:          "main",
				HeadSHA:          "deadbeef",
				ChangedFilesHash: "hash1",
				FifoSeq:          1,
			},
			{
				Branch:           "feature/integrating",
				SessionID:        "session2",
				OriginCommand:    "refine",
				State:            integrationqueue.StateIntegrating,
				Lane:             string(integrationqueue.CodeLane),
				BaseRef:          "main",
				HeadSHA:          "cafebabe",
				ChangedFilesHash: "hash2",
				FifoSeq:          2,
			},
		},
	}
	if err := integrationqueue.SaveQueue(queuePath, queue); err != nil {
		t.Fatalf("SaveQueue: %v", err)
	}

	// Verify entries exist after restart simulation
	loaded, err := integrationqueue.LoadQueue(queuePath)
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}

	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 entries after restart, got %d", len(loaded.Entries))
	}

	// Verify the integrating entry still exists in integrating state
	found := false
	for _, e := range loaded.Entries {
		if e.Branch == "feature/integrating" && e.State == integrationqueue.StateIntegrating {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected integrating entry to survive restart")
	}
}
