package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestApplyReadinessEmergencyOverrideFlagSetsConfig(t *testing.T) {
	readinessEmergencyOverride = true
	defer func() {
		readinessEmergencyOverride = false
	}()

	cfg := &config.Config{}
	applyReadinessEmergencyOverrideFlag(cfg)
	if !cfg.ReadinessEmergencyOverride {
		t.Fatalf("expected readinessEmergencyOverride flag to propagate to config")
	}
}
