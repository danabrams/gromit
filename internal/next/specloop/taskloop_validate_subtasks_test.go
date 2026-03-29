package specloop

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestValidateSubTasks_AllValid(t *testing.T) {
	// Scenario: All sub-tasks have non-empty objectives
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: "refactor parser"},
		{TaskID: "t-006", Objective: "add tests"},
	}

	err := validateSubTasks(subTasks)
	if err != nil {
		t.Fatalf("expected no error for valid objectives, got: %v", err)
	}
}

func TestValidateSubTasks_SingleEmpty(t *testing.T) {
	// Scenario: Single sub-task with empty objective
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: ""},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for empty objective")
	}
	if !strings.Contains(err.Error(), "t-005") {
		t.Fatalf("error should mention t-005, got: %v", err)
	}
}

func TestValidateSubTasks_MultipleEmpty(t *testing.T) {
	// Scenario: Multiple sub-tasks with empty objectives
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: ""},
		{TaskID: "t-006", Objective: ""},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for empty objectives")
	}
	if !strings.Contains(err.Error(), "t-005") {
		t.Fatalf("error should mention t-005, got: %v", err)
	}
	if !strings.Contains(err.Error(), "t-006") {
		t.Fatalf("error should mention t-006, got: %v", err)
	}
}

func TestValidateSubTasks_WhitespaceOnly(t *testing.T) {
	// Scenario: Sub-task with whitespace-only objective
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: "   "},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for whitespace-only objective")
	}
	if !strings.Contains(err.Error(), "t-005") {
		t.Fatalf("error should mention t-005, got: %v", err)
	}
}

func TestValidateSubTasks_MixedValidAndEmpty(t *testing.T) {
	// Scenario: Mix of valid and empty objectives
	// Spec 0004u Scenario 2: all-or-nothing rejection
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: "refactor parser"},
		{TaskID: "t-006", Objective: ""},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for mixed valid and empty objectives")
	}
	if !strings.Contains(err.Error(), "t-006") {
		t.Fatalf("error should mention t-006, got: %v", err)
	}
	// Should NOT mention t-005 since it's valid
	if strings.Contains(err.Error(), "t-005") {
		t.Fatalf("error should not mention valid t-005, got: %v", err)
	}
}

func TestValidateSubTasks_MixedValidAndWhitespace(t *testing.T) {
	// Scenario: Mix of valid and whitespace-only objectives
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: "refactor parser"},
		{TaskID: "t-006", Objective: "  \t\n  "},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for mixed valid and whitespace objectives")
	}
	if !strings.Contains(err.Error(), "t-006") {
		t.Fatalf("error should mention t-006, got: %v", err)
	}
}

func TestValidateSubTasks_Empty(t *testing.T) {
	// Scenario: Empty slice (vacuous truth)
	subTasks := []runstore.Task{}

	err := validateSubTasks(subTasks)
	if err != nil {
		t.Fatalf("expected no error for empty slice, got: %v", err)
	}
}

func TestValidateSubTasks_AllWhitespace(t *testing.T) {
	// Scenario: Multiple sub-tasks with whitespace-only objectives
	subTasks := []runstore.Task{
		{TaskID: "t-005", Objective: "  "},
		{TaskID: "t-006", Objective: "\t"},
		{TaskID: "t-007", Objective: "\n"},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for whitespace-only objectives")
	}
	if !strings.Contains(err.Error(), "t-005") {
		t.Fatalf("error should mention t-005, got: %v", err)
	}
	if !strings.Contains(err.Error(), "t-006") {
		t.Fatalf("error should mention t-006, got: %v", err)
	}
	if !strings.Contains(err.Error(), "t-007") {
		t.Fatalf("error should mention t-007, got: %v", err)
	}
}

func TestValidateSubTasks_ErrorMessageFormat(t *testing.T) {
	// Scenario: Verify error message includes task IDs and is clear
	subTasks := []runstore.Task{
		{TaskID: "t-001", Objective: ""},
		{TaskID: "t-002", Objective: "valid objective"},
		{TaskID: "t-003", Objective: "   "},
	}

	err := validateSubTasks(subTasks)
	if err == nil {
		t.Fatalf("expected error for mixed objectives")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "t-001") {
		t.Fatalf("error should mention t-001, got: %v", err)
	}
	if !strings.Contains(errMsg, "t-003") {
		t.Fatalf("error should mention t-003, got: %v", err)
	}
	if strings.Contains(errMsg, "t-002") {
		t.Fatalf("error should not mention valid t-002, got: %v", err)
	}
}
