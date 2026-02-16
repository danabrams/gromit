//go:build acceptance

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func validationAcceptanceConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{Binary: "claude", Timeout: 300},
		Paths:  config.PathsConfig{Templates: templatesDir},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func TestRunnerAcceptanceSurfaceOnly_ValidationFlow(t *testing.T) {
	cfg := validationAcceptanceConfig(t)
	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner == nil {
		t.Fatal("expected NewRunner to return a runner")
	}

	if err := runner.Run(context.Background(), 1, cfg.TimeBudget, nil, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerAcceptanceReducedCountValidation(t *testing.T) {
	src := readRunnerTestFile(t, "validation_extraction_acceptance_test.go")
	if count := strings.Count(src, "\nfunc Test"); count > 3 {
		t.Fatalf("validation_extraction_acceptance_test.go contains %d tests; expected <= 3", count)
	}
}
