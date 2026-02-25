package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/runner"
)

func TestStatusCmd_OutputIncludesPipelineSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create status.json with a stopped run (PID 0 avoids process-alive checks)
	status := runner.Status{
		Running:   false,
		Iteration: 3,
		BeadID:    "gromit-abc",
		BeadTitle: "Fix login",
		Model:     "sonnet",
		StartedAt: time.Now().Add(-5 * time.Minute),
		ElapsedS:  300,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "status.json"), statusData, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	t.Chdir(tmpDir)

	// Execute status command and capture output
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status command failed: %v", err)
		}
	})

	// When showStatus delegates to runner.PrintStatus, the output includes
	// a Pipeline section. The current manual formatting does not.
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("expected output to contain 'Pipeline:' section (from runner.PrintStatus), got:\n%s", output)
	}
}

func TestStatusCmd_SPCFlagDisplaysPlaceholder(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"status", "--spc"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("status --spc command failed: %v", err)
		}
	})

	const placeholder = "SPC dashboard is not yet implemented"
	if !strings.Contains(output, placeholder) {
		t.Fatalf("expected placeholder %q in output, got:\n%s", placeholder, output)
	}
}

func TestStatusCmd_SPCFlagSkipsDefaultSections(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)

	stdout, stderr, exitCode := runGromitCobra(t, "status", "--spc")
	if exitCode != 0 {
		t.Fatalf("status --spc exit %d, stderr: %s", exitCode, stderr)
	}

	const placeholder = "SPC dashboard is not yet implemented"
	if !strings.Contains(stdout, placeholder) {
		t.Fatalf("expected placeholder %q in output, got:\n%s", placeholder, stdout)
	}

	for _, section := range []string{"Run:", "Pipeline:", "Health:"} {
		if strings.Contains(stdout, section) {
			t.Fatalf("expected SPC guard path to skip %q section, got:\n%s", section, stdout)
		}
	}
}
