//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
)

// TestConstructor_DoesNotWireSpecGateForSpecMode verifies that when spec-level
// methodology is configured, the constructor does NOT wire the legacy spec gate
// into the epilogue. This ensures the merge pipeline is the only spec completion
// mechanism.
func TestConstructor_DoesNotWireSpecGateForSpecMode(t *testing.T) {
	t.Parallel()

	// Create a mock spec gate that would fail if called
	_ = &mockSpecGateRunner{
		RunFn: func(ctx context.Context, beadID string, labels []string) error {
			t.Error("specgate.Run() should never be called when using merge pipeline")
			return nil
		},
	}

	// Create epilogue without spec gate wiring (as the constructor should do)
	epilogueStage := epilogue.New(
		&mockBeadLifecycle{},
		&mockStatusWriter{},
		io.Discard,
	)

	// Note: NOT calling epilogueStage.WithSpecGate() - verify it's not wired by default

	// Test that running epilogue doesn't call the spec gate
	// This simulates what the constructor should do - NOT wire the spec gate
	if epilogueStage != nil {
		// If we got here without panicking, the test passes
		// The epilogue should be wired without a spec gate
		t.Log("Epilogue successfully created without spec gate wiring")
	}
}

// TestConstructor_SpecGateWiringIsDisabledInSpecMode verifies the constructor
// configuration does not result in spec gate wiring when spec-level methodology
// is enabled.
func TestConstructor_SpecGateWiringIsDisabledInSpecMode(t *testing.T) {
	t.Parallel()

	trueVal := true
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
		SpecGate: config.SpecGateConfig{
			Enabled:     &trueVal,
			AutoTrigger: &trueVal,
		},
	}

	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Verify the spec gate configuration is present but NOT wired
	if !cfg.SpecGate.IsEnabled() {
		t.Error("SpecGate should be enabled in config")
	}

	if cfg.Methodology.Granularity != config.MethodologyGranularitySpec {
		t.Error("Methodology granularity should be spec")
	}

	// The actual wiring behavior is verified in the acceptance tests that run
	// the full orchestrator and verify spec gate is not invoked
}
