package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExperimentsCmd_RegistrationAndFlags(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "experiments" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("experiments command not registered")
	}

	jsonFlag := experimentsCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatalf("experiments command should expose --json flag")
	}
	if jsonFlag.Value.Type() != "bool" {
		t.Fatalf("--json flag should be bool, got %s", jsonFlag.Value.Type())
	}
}

func TestExperimentsCmd_ShowsNoExperimentsMessage(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	experimentsDir := filepath.Join(gromitDir, "experiments")
	if err := os.MkdirAll(experimentsDir, 0o755); err != nil {
		t.Fatalf("failed to create experiments dir: %v", err)
	}

	configFile := filepath.Join(tmpDir, "gromit.yaml")
	configContent := "paths:\n  gromit_dir: .gromit\n"
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(origDir)
	})

	restore := configPath
	defer func() { configPath = restore }()
	configPath = configFile

	output := captureExperimentsStdout(t, func() {
		rootCmd.SetArgs([]string{"experiments"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("experiments command failed: %v", err)
		}
	})

	if !strings.Contains(output, "No experiments") {
		t.Fatalf("expected message about missing experiments, got %q", output)
	}
}

func captureExperimentsStdout(t *testing.T, fn func()) string {
	t.Helper()
	if err := experimentsCmd.Flags().Set("json", "false"); err != nil {
		t.Fatalf("failed to reset --json flag: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()

	origStdout := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}
	os.Stdout = origStdout

	return <-done
}
