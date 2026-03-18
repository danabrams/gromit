package planner

import "testing"

func TestPlan_TaskByID(t *testing.T) {
	p := Plan{Tasks: []TaskDef{
		{TaskID: "t-001", Objective: "first"},
		{TaskID: "t-002", Objective: "second"},
	}}
	task, ok := p.TaskByID("t-002")
	if !ok || task.Objective != "second" {
		t.Fatal("expected to find t-002")
	}
	_, ok = p.TaskByID("t-999")
	if ok {
		t.Fatal("should not find nonexistent task")
	}
}

func TestPlan_NormalizeNilFields(t *testing.T) {
	p := Plan{}
	p.NormalizeNilFields()
	if p.Tasks == nil {
		t.Fatal("Tasks should be non-nil after normalize")
	}
	if p.FailuresAddressed == nil {
		t.Fatal("FailuresAddressed should be non-nil after normalize")
	}
}

func TestTaskDef_NormalizeNilFields(t *testing.T) {
	td := TaskDef{}
	td.NormalizeNilFields()
	if td.ExpectedTouchedArea == nil {
		t.Fatal("ExpectedTouchedArea should be non-nil after normalize")
	}
	if td.ProofChecks == nil {
		t.Fatal("ProofChecks should be non-nil after normalize")
	}
	if td.FailuresAddressed == nil {
		t.Fatal("FailuresAddressed should be non-nil after normalize")
	}
}

func TestTaskDef_Fixes(t *testing.T) {
	td := TaskDef{TaskID: "t-001", Objective: "test", Fixes: "fix-001"}
	if td.Fixes != "fix-001" {
		t.Fatal("Fixes field should be populated")
	}
}
