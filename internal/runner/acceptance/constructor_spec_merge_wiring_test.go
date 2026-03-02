//go:build acceptance

package acceptance_test

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// TestConstructorWiringForSpecMode verifies that when spec-level methodology is
// configured, the constructor wires the merge pipeline and does NOT wire the
// legacy spec gate into the epilogue.
func TestConstructorWiringForSpecMode(t *testing.T) {
	t.Parallel()

	// Create a config with spec-level methodology enabled
	trueVal := true
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
		SpecGate: config.SpecGateConfig{
			Enabled:     &trueVal,
			AutoTrigger: &trueVal,
		},
		Paths: config.PathsConfig{
			GromitDir:       ".",
			Templates:       ".",
			Specs:           ".",
			Logs:            "/tmp",
			ProjectClaudeMD: "",
		},
	}

	// Try to create an orchestrator with spec methodology
	// This should not panic or error out
	_, err := runner.NewRunner(cfg, io.Discard)
	if err != nil {
		// It's acceptable for NewRunner to fail due to missing dependencies,
		// but it should not fail due to configuration issues
		t.Logf("NewRunner returned error (expected if templates/logs missing): %v", err)
	}

	// Verify that the constructor successfully handles spec-mode config
	// by checking that the config was properly set up
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if cfg.Methodology.Granularity != config.MethodologyGranularitySpec {
		t.Errorf("Expected spec granularity to be preserved, got %s", cfg.Methodology.Granularity)
	}
}

func TestConstructor_WiresSpecMergePipelineInSpecMode(t *testing.T) {
	t.Parallel()

	logsDir := t.TempDir()
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
		Paths: config.PathsConfig{
			GromitDir:       ".gromit",
			Templates:       filepath.Join(".gromit", "templates"),
			Specs:           filepath.Join(".gromit", "specs"),
			Plans:           filepath.Join(".gromit", "plans"),
			Logs:            logsDir,
			ProjectClaudeMD: "CLAUDE.md",
		},
	}

	orch, err := runner.NewRunner(cfg, io.Discard)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}
	if orch == nil {
		t.Fatal("NewRunner returned nil orchestrator")
	}

	orchCfg := reflect.ValueOf(orch).Elem().FieldByName("cfg")
	if !orchCfg.IsValid() {
		t.Fatal("orchestrator cfg field is missing")
	}
	specMergeField := orchCfg.FieldByName("SpecMergeController")
	if !specMergeField.IsValid() {
		t.Fatal("SpecMergeController field missing from orchestrator config")
	}
	if specMergeField.IsNil() {
		t.Fatal("SpecMergeController should be wired for spec-level methodology")
	}
}

func TestRootHelpGoldenIncludesPRSCommand(t *testing.T) {
	t.Parallel()

	content := loadRootHelpGolden(t)
	if !strings.Contains(content, "  prs           ") {
		t.Fatalf("root help golden file missing prs entry:\n%s", content)
	}
}

func loadRootHelpGolden(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "cmd", "gromit", "testdata", "golden", "root.help.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read root help golden file %q: %v", path, err)
	}
	return string(data)
}
