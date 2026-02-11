package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestModelTimeoutOverrides_InvocationTimeout verifies that per-model invocation
// timeout overrides load from YAML and are returned by TimeoutsForModel.
//
// Expected failure: ModelTimeoutOverrides does not have a Timeout field yet,
// and TimeoutsForModel does not return an invocation timeout value.
func TestModelTimeoutOverrides_InvocationTimeout(t *testing.T) {
	yamlContent := `
claude:
  timeout: 900
  stall_timeout: 180
  stall_timeout_active: 600
  bead_timeout: 1800
  model_timeouts:
    sonnet:
      timeout: 1200
      stall_timeout_active: 900
      bead_timeout: 2400
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Expected failure: Timeout field does not exist on ModelTimeoutOverrides yet
	sonnetOverrides := cfg.Claude.ModelTimeouts["sonnet"]
	if sonnetOverrides.Timeout != 1200 {
		t.Errorf("sonnet invocation timeout: got %d, want 1200", sonnetOverrides.Timeout)
	}
}

// TestTimeoutsForModel_ReturnsInvocationTimeout verifies that TimeoutsForModel
// returns the per-model invocation timeout as a fourth return value.
//
// Expected failure: TimeoutsForModel currently returns 3 values (stall,
// stallActive, bead) — it does not return invocation timeout yet.
func TestTimeoutsForModel_ReturnsInvocationTimeout(t *testing.T) {
	cfg := ClaudeConfig{
		Timeout:            900,
		StallTimeout:       180,
		StallTimeoutActive: 600,
		BeadTimeout:        1800,
		ModelTimeouts: map[string]ModelTimeoutOverrides{
			"sonnet": {
				Timeout: 1200,
			},
		},
	}

	// Expected failure: TimeoutsForModel returns 3 values, not 4
	invocationTimeout, _, _, _ := cfg.TimeoutsForModel("sonnet")
	if invocationTimeout != 1200 {
		t.Errorf("sonnet invocation timeout: got %d, want 1200", invocationTimeout)
	}

	// Model without override should get the base timeout
	invocationTimeout, _, _, _ = cfg.TimeoutsForModel("opus")
	if invocationTimeout != 900 {
		t.Errorf("opus invocation timeout: got %d, want 900 (base default)", invocationTimeout)
	}
}

// TestProjectGromitYAML_HasModelTimeouts loads the actual project gromit.yaml
// and verifies that sonnet and haiku have per-model timeout overrides configured.
//
// Expected failure: gromit.yaml does not have a model_timeouts section yet.
func TestProjectGromitYAML_HasModelTimeouts(t *testing.T) {
	// Find project root by walking up from test directory
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", cfgPath, err)
	}

	t.Run("sonnet_has_overrides", func(t *testing.T) {
		// Expected failure: no model_timeouts section in gromit.yaml yet
		sonnet, ok := cfg.Claude.ModelTimeouts["sonnet"]
		if !ok {
			t.Fatal("gromit.yaml missing model_timeouts entry for sonnet")
		}
		// Sonnet should get a longer invocation timeout than the base 900s
		if sonnet.Timeout <= cfg.Claude.Timeout {
			t.Errorf("sonnet invocation timeout (%d) should be greater than base timeout (%d)",
				sonnet.Timeout, cfg.Claude.Timeout)
		}
	})

	t.Run("haiku_has_overrides", func(t *testing.T) {
		// Expected failure: no model_timeouts section in gromit.yaml yet
		haiku, ok := cfg.Claude.ModelTimeouts["haiku"]
		if !ok {
			t.Fatal("gromit.yaml missing model_timeouts entry for haiku")
		}
		// Haiku should get shorter stall/bead timeouts for faster failure detection
		if haiku.StallTimeout >= cfg.Claude.StallTimeout {
			t.Errorf("haiku stall_timeout (%d) should be less than base stall_timeout (%d)",
				haiku.StallTimeout, cfg.Claude.StallTimeout)
		}
		if haiku.BeadTimeout >= cfg.Claude.BeadTimeout {
			t.Errorf("haiku bead_timeout (%d) should be less than base bead_timeout (%d)",
				haiku.BeadTimeout, cfg.Claude.BeadTimeout)
		}
	})
}

// findProjectRoot walks up from the current working directory to find the
// project root (directory containing gromit.yaml).
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gromit.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no gromit.yaml found)")
		}
		dir = parent
	}
}
