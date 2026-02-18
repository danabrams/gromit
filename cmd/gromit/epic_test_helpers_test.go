package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func prependFakeTools(t *testing.T, testDir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	fakesDir := filepath.Join(wd, "..", "..", "test", "fakes")
	path := os.Getenv("PATH")
	t.Setenv("PATH", fakesDir+string(os.PathListSeparator)+path)
	t.Setenv("TEST_DIR", testDir)
}

// chdirTo changes to dir and restores the original working directory on test cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change directory to %s: %v", dir, err)
	}
}

// writeGromitConfig writes a minimal gromit.yaml to tmpDir configuring the provided path directories.
// Pass specsDir and/or plansDir as empty string to omit them from the config.
func writeGromitConfig(t *testing.T, tmpDir, epicsDir, specsDir, plansDir string) {
	t.Helper()
	config := fmt.Sprintf("paths:\n  gromit_dir: %s\n  epics_dir: %s\n", filepath.Join(tmpDir, ".gromit"), epicsDir)
	if specsDir != "" {
		config += fmt.Sprintf("  specs_dir: %s\n", specsDir)
	}
	if plansDir != "" {
		config += fmt.Sprintf("  plans_dir: %s\n", plansDir)
	}
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write gromit.yaml: %v", err)
	}
}
