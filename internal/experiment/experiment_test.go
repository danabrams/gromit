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
