package planner

import (
	"encoding/json"
	"testing"
)

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

func TestPlan_MarshalJSON_ArchitectureDecisionsPresent(t *testing.T) {
	p := Plan{
		SpecID:                "spec-001",
		Cycle:                 1,
		Tasks:                 []TaskDef{},
		Kind:                  "original",
		ArchitectureDecisions: []string{"decision-1", "decision-2"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, ok := m["architecture_decisions"]; !ok {
		t.Fatal("architecture_decisions key should be present in JSON")
	}
}

func TestPlan_MarshalJSON_ArchitectureDecisionsOmitted(t *testing.T) {
	p := Plan{
		SpecID:                "spec-001",
		Cycle:                 1,
		Tasks:                 []TaskDef{},
		Kind:                  "original",
		ArchitectureDecisions: []string{},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	if _, ok := m["architecture_decisions"]; ok {
		t.Fatal("architecture_decisions key should be omitted in JSON when empty")
	}
}

func TestPlan_NormalizeNilFields_ArchitectureDecisions(t *testing.T) {
	p := Plan{
		SpecID: "spec-001",
		Cycle:  1,
		Tasks:  []TaskDef{},
		Kind:   "original",
	}
	if p.ArchitectureDecisions != nil {
		t.Fatal("ArchitectureDecisions should be nil before normalize")
	}
	p.NormalizeNilFields()
	if p.ArchitectureDecisions == nil {
		t.Fatal("ArchitectureDecisions should be non-nil after normalize")
	}
	if len(p.ArchitectureDecisions) != 0 {
		t.Fatal("ArchitectureDecisions should be empty slice after normalize")
	}
}

func TestPlan_UnmarshalJSON_ArchitectureDecisions(t *testing.T) {
	jsonData := []byte(`{
		"spec_id": "spec-001",
		"cycle": 1,
		"tasks": [],
		"kind": "original",
		"architecture_decisions": ["decision-1", "decision-2", "decision-3"]
	}`)
	var p Plan
	if err := json.Unmarshal(jsonData, &p); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(p.ArchitectureDecisions) != 3 {
		t.Fatalf("expected 3 architecture decisions, got %d", len(p.ArchitectureDecisions))
	}
	if p.ArchitectureDecisions[0] != "decision-1" || p.ArchitectureDecisions[1] != "decision-2" || p.ArchitectureDecisions[2] != "decision-3" {
		t.Fatal("architecture decisions values don't match after unmarshal")
	}

	// Round-trip test
	marshaled, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var p2 Plan
	if err := json.Unmarshal(marshaled, &p2); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}
	if len(p2.ArchitectureDecisions) != 3 {
		t.Fatalf("expected 3 architecture decisions after round-trip, got %d", len(p2.ArchitectureDecisions))
	}
}
