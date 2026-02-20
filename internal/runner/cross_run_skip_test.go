package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
)

func TestInitRunLoopState_SkipsBeadsWithConsecutiveFailures(t *testing.T) {
	tmpDir := t.TempDir()
	metricsDir := filepath.Join(tmpDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		t.Fatalf("create metrics dir: %v", err)
	}
	metrics := `{"bead_id":"bead-123","success":false}
{"bead_id":"bead-123","success":false}
{"bead_id":"bead-ok","success":false}
{"bead_id":"bead-ok","success":true}
`
	if err := os.WriteFile(filepath.Join(metricsDir, "iteration_metrics.jsonl"), []byte(metrics), 0644); err != nil {
		t.Fatalf("write metrics file: %v", err)
	}

	cfg := &config.Config{
		Loop:  config.LoopConfig{MaxCrossRunFailures: 2},
		Paths: config.PathsConfig{Logs: filepath.Join(tmpDir, "logs")},
	}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{Renderer: &mockPromptRenderer{}})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps error: %v", err)
	}

	st, cleanup, err := r.initRunLoopState(time.Time{})
	if err != nil {
		t.Fatalf("initRunLoopState error: %v", err)
	}
	cleanup()

	if !st.skippedBeads["bead-123"] {
		t.Fatalf("expected bead-123 to be skipped")
	}
	if st.skippedBeads["bead-ok"] {
		t.Fatalf("did not expect bead-ok to be skipped")
	}

	output := buf.String()
	if !strings.Contains(output, "bead-123") || !strings.Contains(output, "consecutive failures") {
		t.Fatalf("expected warning about consecutive failures for bead-123, got: %s", output)
	}
}

func TestInitRunLoopState_UsesInjectedGitHeadFnForReviewBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{Logs: filepath.Join(tmpDir, "logs")},
	}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{Renderer: &mockPromptRenderer{}})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps error: %v", err)
	}
	r.gitHeadFn = func() (string, error) {
		return "abc123", nil
	}

	st, cleanup, err := r.initRunLoopState(time.Time{})
	if err != nil {
		t.Fatalf("initRunLoopState error: %v", err)
	}
	cleanup()

	if st.interactiveFile == nil {
		t.Fatal("expected interactive file to be initialized")
	}
	if st.interactiveFile.LastReviewCommit() != "abc123" {
		t.Fatalf("LastReviewCommit = %q, want %q", st.interactiveFile.LastReviewCommit(), "abc123")
	}
}
