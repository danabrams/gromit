package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
)

// TestProcessSingleBead_ShowError_SkipsBead verifies that when beads.Show()
// returns an error, the bead is added to skippedBeads and processSingleBead
// returns false, nil (skip without error) instead of continuing to process.
func TestProcessSingleBead_ShowError_SkipsBead(t *testing.T) {
	cfg := &config.Config{
		Loop: config.LoopConfig{
			StuckBeadThreshold: 10, // high threshold so stuck check doesn't trigger
		},
	}

	mockBeads := &mockBeadClient{
		ShowFn: func(id string) (*bead.Bead, error) {
			return nil, fmt.Errorf("bd show failed: connection timeout")
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    mockBeads,
		Renderer: &mockPromptRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps error: %v", err)
	}

	b := &bead.Bead{
		ID:              "bead-123",
		Title:           "Test bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	st := &runLoopState{
		beadStats:    make(map[string]logger.BeadStats),
		skippedBeads: make(map[string]bool),
	}

	shouldStop, err := r.processSingleBead(
		context.Background(),
		b,
		st,
		10,           // maxIterations
		time.Time{},  // deadline (zero = no deadline)
		false,        // dryRun
		nil,          // tmuxMgr
		func(int) {}, // runThoroughReview
	)

	if err != nil {
		t.Fatalf("processSingleBead returned error: %v", err)
	}
	if shouldStop {
		t.Fatal("processSingleBead returned shouldStop=true, want false")
	}
	if !st.skippedBeads["bead-123"] {
		t.Fatal("expected bead-123 to be in skippedBeads after Show error")
	}

	output := buf.String()
	if !strings.Contains(output, "skipping as precaution") {
		t.Fatalf("expected warning about skipping as precaution, got: %s", output)
	}
}

// TestProcessSingleBead_ShowReturnsClosedBead_SkipsBead verifies the existing
// behavior that a bead returned as closed by Show is skipped.
func TestProcessSingleBead_ShowReturnsClosedBead_SkipsBead(t *testing.T) {
	cfg := &config.Config{
		Loop: config.LoopConfig{
			StuckBeadThreshold: 10,
		},
	}

	mockBeads := &mockBeadClient{
		ShowFn: func(id string) (*bead.Bead, error) {
			return &bead.Bead{
				ID:              id,
				Title:           "Test bead",
				Priority:        1,
				Status:          "closed",
				Labels:          []string{},
				ExpectedOutputs: []string{},
			}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    mockBeads,
		Renderer: &mockPromptRenderer{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps error: %v", err)
	}

	b := &bead.Bead{
		ID:              "bead-456",
		Title:           "Already closed bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	st := &runLoopState{
		beadStats:    make(map[string]logger.BeadStats),
		skippedBeads: make(map[string]bool),
	}

	shouldStop, err := r.processSingleBead(
		context.Background(),
		b,
		st,
		10,
		time.Time{},
		false,
		nil,
		func(int) {},
	)

	if err != nil {
		t.Fatalf("processSingleBead returned error: %v", err)
	}
	if shouldStop {
		t.Fatal("processSingleBead returned shouldStop=true, want false")
	}
	if !st.skippedBeads["bead-456"] {
		t.Fatal("expected bead-456 to be in skippedBeads for closed bead")
	}
}
