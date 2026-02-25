package experiment

import (
	"testing"
	"time"
)

func TestExperimentStructHasAllRequiredFields(t *testing.T) {
	// Verify that Experiment struct can be created with all fields
	exp := &Experiment{
		ID:               "test-exp-1",
		Phase:            "build",
		Description:      "Test experiment",
		Created:          time.Now(),
		Control:          &Variant{},
		Variants:         []*Variant{},
		SuccessCriteria:  "",
		ForceVariant:     "",
	}

	if exp.ID != "test-exp-1" {
		t.Fatalf("Expected ID to be 'test-exp-1', got %q", exp.ID)
	}
	if exp.Phase != "build" {
		t.Fatalf("Expected Phase to be 'build', got %q", exp.Phase)
	}
}

func TestNewManagerConstructsManagerWithExperiments(t *testing.T) {
	// Test NewManager constructor with a slice of experiments
	exp1 := &Experiment{
		ID:    "exp-1",
		Phase: "build",
	}
	exp2 := &Experiment{
		ID:    "exp-2",
		Phase: "validate",
	}

	mgr := NewManager([]*Experiment{exp1, exp2}, "/test/state")

	if mgr == nil {
		t.Fatalf("NewManager returned nil")
	}

	if mgr.stateDir != "/test/state" {
		t.Fatalf("Expected stateDir to be '/test/state', got %q", mgr.stateDir)
	}
}

func TestExperimentForPhaseReturnsExperimentOrNil(t *testing.T) {
	// Test that ExperimentForPhase returns the correct experiment or nil
	exp1 := &Experiment{
		ID:    "exp-1",
		Phase: "build",
	}
	exp2 := &Experiment{
		ID:    "exp-2",
		Phase: "validate",
	}

	mgr := NewManager([]*Experiment{exp1, exp2}, "/test/state")

	// Test getting an experiment that exists
	buildExp := mgr.ExperimentForPhase("build")
	if buildExp == nil {
		t.Fatalf("Expected to find experiment for 'build' phase")
	}
	if buildExp.ID != "exp-1" {
		t.Fatalf("Expected exp-1 for 'build' phase, got %q", buildExp.ID)
	}

	// Test getting an experiment for a phase with no experiment
	refactorExp := mgr.ExperimentForPhase("refactor")
	if refactorExp != nil {
		t.Fatalf("Expected nil for 'refactor' phase with no experiment")
	}
}
