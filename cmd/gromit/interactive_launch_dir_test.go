package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeStatusJSON(t *testing.T, gromitDir string, running bool, pid int) {
	t.Helper()
	status := map[string]interface{}{
		"running": running,
		"pid":     pid,
	}
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), data, 0644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
}

func makeGromitDir(t *testing.T) (mainDir, gromitDir string) {
	t.Helper()
	mainDir = t.TempDir()
	gromitDir = filepath.Join(mainDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("create gromit dir: %v", err)
	}
	return mainDir, gromitDir
}

func TestInteractiveLaunchDir_Active(t *testing.T) {
	mainDir, gromitDir := makeGromitDir(t)
	writeStatusJSON(t, gromitDir, true, os.Getpid())

	got := interactiveLaunchDir(gromitDir)

	want := mainDir + "-gromit-interactive"
	if got != want {
		t.Errorf("interactiveLaunchDir() = %q, want %q", got, want)
	}
}

func TestInteractiveLaunchDir_Inactive(t *testing.T) {
	_, gromitDir := makeGromitDir(t)
	writeStatusJSON(t, gromitDir, false, os.Getpid())

	got := interactiveLaunchDir(gromitDir)

	if got != "" {
		t.Errorf("interactiveLaunchDir() = %q, want empty string (inactive run loop)", got)
	}
}

func TestInteractiveLaunchDir_Stale(t *testing.T) {
	_, gromitDir := makeGromitDir(t)
	// running=true but dead PID
	writeStatusJSON(t, gromitDir, true, 999999999)

	got := interactiveLaunchDir(gromitDir)

	if got != "" {
		t.Errorf("interactiveLaunchDir() = %q, want empty string (stale run loop)", got)
	}
}

func TestInteractiveLaunchDir_Missing(t *testing.T) {
	_, gromitDir := makeGromitDir(t)
	// No status.json written

	got := interactiveLaunchDir(gromitDir)

	if got != "" {
		t.Errorf("interactiveLaunchDir() = %q, want empty string (missing status.json)", got)
	}
}

func TestInteractiveLaunchDir_InvalidStatus(t *testing.T) {
	_, gromitDir := makeGromitDir(t)
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}

	got := interactiveLaunchDir(gromitDir)

	if got != "" {
		t.Errorf("interactiveLaunchDir() = %q, want empty string (invalid status.json)", got)
	}
}
