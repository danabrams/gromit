package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
)

func TestPipelineListStuckReturnsStuckBeads(t *testing.T) {
	t.Parallel()

	logsDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	writeLogEntries(t, logsDir, "stuck-1", []time.Time{now, now.Add(time.Minute)})

	stuckBead := &bead.Bead{ID: "stuck-1", Title: "Stuck bead", Priority: 1}
	client := &fakeQueueClient{
		listReadyWorkFn: func(ctx context.Context) ([]*bead.Bead, error) {
			return []*bead.Bead{stuckBead}, nil
		},
		listFn: func(ctx context.Context) ([]*bead.Bead, error) {
			return []*bead.Bead{stuckBead}, nil
		},
		listByStatusFn: func(ctx context.Context, status string) ([]*bead.Bead, error) {
			if status == "in_progress" {
				return nil, nil
			}
			return nil, nil
		},
	}

	p := &Pipeline{deps: &Deps{QueueClient: client}}
	stuck, err := p.ListStuck(context.Background(), QueueInput{LogsDir: logsDir, StuckThreshold: 2})
	if err != nil {
		t.Fatalf("ListStuck returned error: %v", err)
	}

	if got, want := len(stuck), 1; got != want {
		t.Fatalf("expected %d stuck bead, got %d", want, got)
	}
	if stuck[0].ID != "stuck-1" {
		t.Fatalf("expected stuck bead ID stuck-1, got %s", stuck[0].ID)
	}
}

func TestPipelineUnstickErrorsWhenBeadMissing(t *testing.T) {
	t.Parallel()

	client := &fakeQueueClient{
		listFn: func(ctx context.Context) ([]*bead.Bead, error) {
			return []*bead.Bead{{ID: "other"}}, nil
		},
		listByStatusFn: func(ctx context.Context, status string) ([]*bead.Bead, error) {
			return nil, nil
		},
	}

	p := &Pipeline{deps: &Deps{QueueClient: client}}
	err := p.Unstick(context.Background(), "missing", t.TempDir())
	if err == nil {
		t.Fatal("expected error when bead does not exist")
	}
}

func TestPipelineUnstickWritesRestartPointAndEmitsEvent(t *testing.T) {
	t.Parallel()

	gromitDir := t.TempDir()

	client := &fakeQueueClient{
		listFn: func(ctx context.Context) ([]*bead.Bead, error) {
			return []*bead.Bead{{ID: "b1", Title: "Stuck bead"}}, nil
		},
		listByStatusFn: func(ctx context.Context, status string) ([]*bead.Bead, error) {
			return nil, nil
		},
	}

	emitter := &fakeEmitter{}
	p := &Pipeline{
		deps:    &Deps{QueueClient: client},
		emitter: emitter,
	}

	if err := p.Unstick(context.Background(), "b1", gromitDir); err != nil {
		t.Fatalf("unexpected Unstick error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(gromitDir, "restart-points.json"))
	if err != nil {
		t.Fatalf("expected restart points file: %v", err)
	}

	var points map[string]struct {
		RestartAt time.Time `json:"restart_at"`
		Reason    string    `json:"reason"`
	}
	if err := json.Unmarshal(data, &points); err != nil {
		t.Fatalf("failed to unmarshal restart points: %v", err)
	}

	pt, ok := points["b1"]
	if !ok {
		t.Fatalf("expected restart point for bead b1")
	}
	if pt.Reason != "manual" {
		t.Fatalf("unexpected restart reason: %s", pt.Reason)
	}
	if pt.RestartAt.IsZero() {
		t.Fatalf("expected restart time to be set")
	}

	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	evt, ok := emitter.events[0].(*events.BeadUnstickedEvent)
	if !ok {
		t.Fatalf("expected BeadUnstickedEvent, got %T", emitter.events[0])
	}
	if evt.BeadID != "b1" {
		t.Fatalf("unexpected bead ID: %s", evt.BeadID)
	}
	if evt.Reason != "manual" {
		t.Fatalf("unexpected reason: %s", evt.Reason)
	}
}

type fakeEmitter struct {
	events []events.Event
}

func (f *fakeEmitter) Emit(evt events.Event) {
	if f == nil {
		return
	}
	f.events = append(f.events, evt)
}

func writeLogEntries(t *testing.T, logsDir, beadID string, timestamps []time.Time) {
	t.Helper()

	lines := make([]string, 0, len(timestamps))
	for i, ts := range timestamps {
		lines = append(lines, fmt.Sprintf(
			`{"timestamp":"%s","iteration":%d,"bead_id":"%s","bead_title":"Test","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":%d}`,
			ts.Format(time.RFC3339),
			i+1,
			beadID,
			1+i,
		))
	}

	filename := filepath.Join(logsDir, "run-20260301-000000.jsonl")
	if err := os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}
}
