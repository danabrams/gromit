package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestSynthesizeFixBeads_ZeroFailures(t *testing.T) {
	t.Helper()

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", nil, nil)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want empty", ids)
	}
}

func TestSynthesizeFixBeads_SingleFailure(t *testing.T) {
	t.Helper()

	failures := []GateFailure{{
		TestName:     "TestSpecGate",
		Message:      "expected 200 got 500",
		SuggestedFix: "handle nil response",
	}}

	var gotTitle string
	var gotPriority int
	var gotLabels []string
	var gotExpectedOutputs []string

	beads := &mockBeadClient{
		CreateFn: func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
			gotTitle = title
			gotPriority = priority
			gotLabels = append([]string{}, labels...)
			gotExpectedOutputs = append([]string{}, expectedOutputs...)
			return &bead.Bead{ID: "fix-1"}, nil
		},
	}

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", failures, beads)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "fix-1" {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want [fix-1]", ids)
	}
	if gotTitle != "Fix: TestSpecGate" {
		t.Fatalf("Create title = %q, want %q", gotTitle, "Fix: TestSpecGate")
	}
	if gotPriority != 0 {
		t.Fatalf("Create priority = %d, want 0", gotPriority)
	}
	if len(gotLabels) != 1 || gotLabels[0] != "spec:alpha" {
		t.Fatalf("Create labels = %v, want [spec:alpha]", gotLabels)
	}
	if len(gotExpectedOutputs) != 1 {
		t.Fatalf("Create expectedOutputs = %v, want 1 item", gotExpectedOutputs)
	}
	description := gotExpectedOutputs[0]
	if !strings.Contains(description, "Criterion: TestSpecGate") {
		t.Fatalf("description = %q, missing criterion name", description)
	}
	if !strings.Contains(description, "Evidence: expected 200 got 500") {
		t.Fatalf("description = %q, missing evidence", description)
	}
	if !strings.Contains(description, "Suggested fix: handle nil response") {
		t.Fatalf("description = %q, missing suggested fix", description)
	}
	if !strings.Contains(description, "Fix direction:") {
		t.Fatalf("description = %q, missing fix direction", description)
	}
}

func TestSynthesizeFixBeads_CapsAtFive(t *testing.T) {
	t.Helper()

	failures := []GateFailure{
		{TestName: "one", Message: "m1"},
		{TestName: "two", Message: "m2"},
		{TestName: "three", Message: "m3"},
		{TestName: "four", Message: "m4"},
		{TestName: "five", Message: "m5"},
		{TestName: "six", Message: "m6"},
	}

	createCalls := 0
	beads := &mockBeadClient{
		CreateFn: func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
			createCalls++
			return &bead.Bead{ID: title}, nil
		},
	}

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", failures, beads)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if createCalls != runtypes.MaxSynthesizedSpecFixBeads {
		t.Fatalf("Create calls = %d, want %d", createCalls, runtypes.MaxSynthesizedSpecFixBeads)
	}
	if len(ids) != runtypes.MaxSynthesizedSpecFixBeads {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want %d ids", ids, runtypes.MaxSynthesizedSpecFixBeads)
	}
}

func TestSynthesizeFixBeads_ReturnsCreateErrors(t *testing.T) {
	t.Helper()

	failures := []GateFailure{
		{TestName: "one", Message: "m1"},
		{TestName: "two", Message: "m2"},
	}

	call := 0
	beads := &mockBeadClient{
		CreateFn: func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
			call++
			if call == 1 {
				return &bead.Bead{ID: "fix-1"}, nil
			}
			return nil, errors.New("create failed")
		},
	}

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", failures, beads)
	if err == nil {
		t.Fatal("SynthesizeFixBeads() error = nil, want create failure")
	}
	if len(ids) != 1 || ids[0] != "fix-1" {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want [fix-1]", ids)
	}
	if !strings.Contains(err.Error(), "create bead for criterion \"two\"") {
		t.Fatalf("error = %v, want criterion context", err)
	}
}
