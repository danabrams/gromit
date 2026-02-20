package runner

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestNewRunnerWiresConfigBackedPolicies(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
		},
		Claude: config.ClaudeConfig{Binary: "claude"},
	}

	r, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if r.stuckPolicy == nil {
		t.Fatal("expected NewRunner to wire stuckPolicy")
	}
	if r.escalationPolicy == nil {
		t.Fatal("expected NewRunner to wire escalationPolicy")
	}
	if r.validationPolicy == nil {
		t.Fatal("expected NewRunner to wire validationPolicy")
	}
	if r.methodologyPolicy == nil {
		t.Fatal("expected NewRunner to wire methodologyPolicy")
	}
}

func TestNewRunnerWithDepsDefaultsStuckPolicy(t *testing.T) {
	var buf strings.Builder
	r, err := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(), Deps{})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}
	if r.stuckPolicy == nil {
		t.Fatal("expected NewRunnerWithDeps to default stuckPolicy when deps omit it")
	}
}
