package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
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
