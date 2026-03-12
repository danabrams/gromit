package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestRealStageProvider_BuildStages_ReturnsStages(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages returned error: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("expected at least one stage, got 0")
	}

	expectedNames := []string{"init", "compile", "plan", "execute", "validate", "evidence", "finalize"}
	if len(stages) != len(expectedNames) {
		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
	}
	for i, name := range expectedNames {
		if stages[i].Name() != name {
			t.Errorf("stage[%d].Name() = %q, want %q", i, stages[i].Name(), name)
		}
	}
}

func TestRealStageProvider_BuildStages_NoStubError(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	_, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages should not return stub error, got: %v", err)
	}
}
