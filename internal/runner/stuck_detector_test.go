package runner

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
	"github.com/danabrams/gromit/internal/runner/policy"
)

func TestStuckDetectorAdapter_MarksStuckWithoutRestartPoints(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	logsDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	writeLogEntries(t, logsDir, "b1", []time.Time{now, now.Add(time.Minute)})

	adapter := &stuckDetectorAdapter{
		logsDir:   logsDir,
		gromitDir: tmp,
		policy:    policy.NewThresholdStuckPolicy(2),
	}

	stuck, err := adapter.IsStuck(context.Background(), &bead.Bead{ID: "b1"})
	if err != nil {
		t.Fatalf("IsStuck error: %v", err)
	}
	if !stuck {
		t.Fatalf("expected bead to be stuck without restart point")
	}
}

func TestStuckDetectorAdapter_RespectsRestartPoints(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	logsDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	writeLogEntries(t, logsDir, "b1", []time.Time{now, now.Add(time.Minute)})
	writeRestartPoints(t, tmp, map[string]time.Time{"b1": now.Add(30 * time.Second)})

	adapter := &stuckDetectorAdapter{
		logsDir:   logsDir,
		gromitDir: tmp,
		policy:    policy.NewThresholdStuckPolicy(2),
	}

	stuck, err := adapter.IsStuck(context.Background(), &bead.Bead{ID: "b1"})
	if err != nil {
		t.Fatalf("IsStuck error: %v", err)
	}
	if stuck {
		t.Fatalf("expected bead to be unstuck after restart point")
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
			1000+i,
		))
	}
	filename := filepath.Join(logsDir, "run-20260301-000000.jsonl")
	if err := os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}
}

func writeRestartPoints(t *testing.T, gromitDir string, points map[string]time.Time) {
	t.Helper()
	type point struct {
		RestartAt time.Time `json:"restart_at"`
		Reason    string    `json:"reason"`
	}

	payload := make(map[string]point)
	for id, at := range points {
		payload[id] = point{RestartAt: at, Reason: "manual"}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal restart points: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "restart-points.json"), data, 0644); err != nil {
		t.Fatalf("failed to write restart points: %v", err)
	}
}
