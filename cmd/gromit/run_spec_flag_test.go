package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupRunSpecTestEnv creates a minimal test environment for run command spec flag tests.
func setupRunSpecTestEnv(t *testing.T) (specsDir string, cleanup func()) {
	t.Helper()

	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	specsDir = filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Write minimal gromit.yaml (empty YAML uses all defaults)
	cfgPath := filepath.Join(tempDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write gromit.yaml: %v", err)
	}

	origConfigPath := configPath
	origRunSpec := runSpecFlag
	origRunEpic := runEpicFlag

	t.Chdir(tempDir)

	configPath = "gromit.yaml"

	cleanup = func() {
		configPath = origConfigPath
		runSpecFlag = origRunSpec
		runEpicFlag = origRunEpic
	}

	return specsDir, cleanup
}

// TestRunCmd_ScopeFlagsRegistered verifies that --spec and --epic flags are registered.
func TestRunCmd_ScopeFlagsRegistered(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "spec"},
		{name: "epic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := runCmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("run command should have --%s flag", tt.name)
			}
			if flag.Value.Type() != "string" {
				t.Errorf("--%s flag should be string, got %s", tt.name, flag.Value.Type())
			}
		})
	}
}

func TestRunLoop_ScopeFlagsMutuallyExclusive(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	runSpecFlag = "auth"
	runEpicFlag = "gromit-123"

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop should fail when --spec and --epic are both set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got: %v", err)
	}
}

// TestRunLoop_SpecFlagNonexistentSpec verifies that --spec with a nonexistent spec
// returns a validation error before attempting to run.
func TestRunLoop_SpecFlagNonexistentSpec(t *testing.T) {
	specsDir, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Create some spec files so the error can list them
	for _, name := range []string{"auth", "profile"} {
		path := filepath.Join(specsDir, name+".md")
		if err := os.WriteFile(path, []byte("# "+name), 0644); err != nil {
			t.Fatalf("Failed to write spec: %v", err)
		}
	}

	runSpecFlag = "nonexistent-spec"
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})

	if err == nil {
		t.Fatal("runLoop with nonexistent --spec should return error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "not found") {
		t.Errorf("Error should say 'not found', got: %v", err)
	}
	if !strings.Contains(errMsg, "nonexistent-spec") {
		t.Errorf("Error should mention the requested spec name, got: %v", err)
	}
}

// TestRunLoop_SpecFlagValidSpec verifies that a valid --spec passes validation
// and that the error is NOT a spec validation error (i.e., we proceed to the runner).
func TestRunLoop_SpecFlagValidSpec(t *testing.T) {
	specsDir, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Create the spec file
	specPath := filepath.Join(specsDir, "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	runSpecFlag = "auth"
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})

	// May fail for other reasons (no bd cli, etc), but must NOT fail due to spec validation
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "validating spec:") {
			t.Errorf("Error should not be spec validation error for existing spec, got: %v", err)
		}
		if strings.Contains(errMsg, "Available specs") {
			t.Errorf("Error should not list available specs for valid spec, got: %v", err)
		}
	}
}

func TestRunLoop_EpicFlagMissingSpecsDir(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Remove .gromit/specs to force ResolveEpic error path.
	if err := os.RemoveAll(filepath.Join(".gromit", "specs")); err != nil {
		t.Fatalf("failed removing specs dir: %v", err)
	}

	runSpecFlag = ""
	runEpicFlag = "gromit-123"

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop with --epic and missing specs dir should return error")
	}
	if !strings.Contains(err.Error(), "resolving epic scope:") {
		t.Fatalf("expected epic resolution error, got: %v", err)
	}
}

func TestRunLoop_EpicFlagValidScope(t *testing.T) {
	specsDir, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	specContent := `---
id: auth
epic: gromit-123
---

# Auth spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "auth.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("failed writing spec file: %v", err)
	}

	runSpecFlag = ""
	runEpicFlag = "gromit-123"

	err := runLoop(runCmd, []string{})
	if err != nil && strings.Contains(err.Error(), "resolving epic scope:") {
		t.Fatalf("runLoop should not fail epic scope resolution for a valid epic: %v", err)
	}
}
