package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/state"
)

func TestRunnerCheckRetroSuggestion_UsesInteractiveState(t *testing.T) {
	gromitDir := setupInteractiveStateTestDir(t)
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	writeStateFileForTest(t, gromitDir, state.State{LastRetro: time.Now().Add(-8 * 24 * time.Hour)})
	writeInteractiveStateFileForTest(t, gromitDir, state.InteractiveState{LastRetro: time.Now().Add(-2 * 24 * time.Hour)})

	cfg := &config.Config{
		Paths: config.PathsConfig{Logs: logsDir},
	}

	var buf strings.Builder
	r := &Runner{
		cfg:       cfg,
		gromitDir: gromitDir,
		output:    &buf,
	}

	r.checkRetroSuggestion()

	if strings.Contains(buf.String(), "Retro suggested:") {
		t.Fatalf("expected no retro suggestion when interactive-state.json is recent, got output:\n%s", buf.String())
	}
}

func setupInteractiveStateTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("mkdir gromit dir: %v", err)
	}
	return gromitDir
}

func writeStateFileForTest(t *testing.T, gromitDir string, st state.State) {
	t.Helper()
	path := filepath.Join(gromitDir, "state.json")
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write state file: %v", err)
	}
}

func writeInteractiveStateFileForTest(t *testing.T, gromitDir string, st state.InteractiveState) {
	t.Helper()
	path := filepath.Join(gromitDir, "interactive-state.json")
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("marshal interactive state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write interactive state file: %v", err)
	}
}
