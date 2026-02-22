//go:build acceptance

package acceptance_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestConfigModelsUsesModelsConfigType(t *testing.T) {
	// Verify that Config.Models uses ModelsConfig (not ModelConfig).
	// This mirrors the pattern in invocation_timeout_acceptance_test.go.
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P1: "sonnet",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if cfg.Models.P1 != "sonnet" {
		t.Fatalf("Models.P1 = %q, want %q", cfg.Models.P1, "sonnet")
	}
}
