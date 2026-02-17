//go:build e2e_live
// This test runs the real Claude CLI and is intentionally excluded from the
// default acceptance suite for speed and reliability.

package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
)

func validationAcceptanceConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	copyRunnerTemplates(t, templatesDir)
	logsDir := filepath.Join(tmpDir, ".gromit", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{Binary: "claude", Timeout: 300},
		Paths:  config.PathsConfig{Templates: templatesDir, Logs: logsDir},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func copyRunnerTemplates(t *testing.T, templatesDir string) {
	t.Helper()

	projectRoot := runnerSmokeSuiteRepoRoot(t)
	sourceDir := filepath.Join(projectRoot, ".gromit", "templates")
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		sourcePath := filepath.Join(sourceDir, name)
		destPath := filepath.Join(templatesDir, name)
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read template %s: %v", name, err)
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}
}

// smoke-matrix: keep | rationale: Preserves core end-to-end success path that validates runner wiring from construction through a full run invocation. | destination: internal/runner/validation_extraction_acceptance_test.go:TestRunnerSmoke_RunSingleBeadHappyPath
func TestRunnerSmoke_RunSingleBeadHappyPath(t *testing.T) {
	cfg := validationAcceptanceConfig(t)
	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner == nil {
		t.Fatal("expected NewRunner to return a runner")
	}

	if err := runner.Run(context.Background(), 1, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
