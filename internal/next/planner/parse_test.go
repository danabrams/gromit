package planner

import "testing"

func TestParsePlan_ExtractsJSONFromMarkdown(t *testing.T) {
	raw := "Here is the plan:\n```json\n" +
		`{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"do it","expected_touched_area":["a/"],"proof_checks":["go test ./a/"]}]}` +
		"\n```\nDone."
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
}

func TestParsePlan_BareJSON(t *testing.T) {
	raw := `{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["b/"],"proof_checks":["true"]}]}`
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SpecID != "s1" {
		t.Fatalf("want s1, got %s", plan.SpecID)
	}
}

func TestParsePlan_InvalidJSON(t *testing.T) {
	_, err := ParsePlan("not json at all")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePlan_JSONPrecededByProse(t *testing.T) {
	raw := "Here is the plan:\n" +
		`{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"do it","expected_touched_area":["a/"],"proof_checks":["go test ./a/"]}]}`
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
}

func TestParsePlan_TruncatedFence(t *testing.T) {
	raw := "```json\n" +
		`{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"do it","expected_touched_area":["a/"],"proof_checks":["go test ./a/"]}]}`
	// No closing ``` fence — the balanced-brace fallback should handle this.
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SpecID != "s1" {
		t.Fatalf("want s1, got %s", plan.SpecID)
	}
}

func TestParsePlan_JSONEmbeddedInProse(t *testing.T) {
	raw := "I've analyzed the spec. Here is my plan:\n" +
		`{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"do it","expected_touched_area":["a/"],"proof_checks":["go test ./a/"]}]}` +
		"\nLet me know if you want changes."
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
}
