package contract

import (
	"strings"
	"testing"
)

func TestParseContractYAML_ValidYAML(t *testing.T) {
	input := `scenarios:
  - name: add-works
    assertions:
      - file_exists: calc/calc.go`
	c, err := ParseContractYAML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(c.Scenarios))
	}
	if c.Scenarios[0].Name != "add-works" {
		t.Fatalf("expected scenario name 'add-works', got %q", c.Scenarios[0].Name)
	}
	if len(c.Scenarios[0].Assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(c.Scenarios[0].Assertions))
	}
	if c.Scenarios[0].Assertions[0].FileExists != "calc/calc.go" {
		t.Fatalf("expected FileExists='calc/calc.go', got %q", c.Scenarios[0].Assertions[0].FileExists)
	}
}

func TestParseContractYAML_StripsYAMLFence(t *testing.T) {
	output := "Here is the YAML:\n```yaml\nscenarios:\n  - name: test\n    assertions:\n      - file_exists: foo.go\n```\n"
	c, err := ParseContractYAML(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Scenarios) != 1 || c.Scenarios[0].Name != "test" {
		t.Fatalf("expected 1 scenario named 'test', got %v", c.Scenarios)
	}
}

func TestParseContractYAML_StripsGenericFence(t *testing.T) {
	output := "```\nscenarios:\n  - name: sub-works\n    assertions:\n      - file_not_exists: old.go\n```"
	c, err := ParseContractYAML(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Scenarios) != 1 || c.Scenarios[0].Name != "sub-works" {
		t.Fatalf("expected 1 scenario named 'sub-works', got %v", c.Scenarios)
	}
}

func TestParseContractYAML_AllAssertionTypes(t *testing.T) {
	input := `scenarios:
  - name: all-types
    assertions:
      - file_exists: a.go
      - file_not_exists: b.go
      - file_not_modified: c.go
      - file_contains:
          path: d.go
          pattern: hello
      - file_not_contains:
          path: e.go
          pattern: world`
	c, err := ParseContractYAML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(c.Scenarios))
	}
	assertions := c.Scenarios[0].Assertions
	if len(assertions) != 5 {
		t.Fatalf("expected 5 assertions, got %d", len(assertions))
	}
	if assertions[0].FileExists != "a.go" {
		t.Errorf("assertion[0]: expected FileExists='a.go', got %q", assertions[0].FileExists)
	}
	if assertions[1].FileNotExists != "b.go" {
		t.Errorf("assertion[1]: expected FileNotExists='b.go', got %q", assertions[1].FileNotExists)
	}
	if assertions[2].FileNotModified != "c.go" {
		t.Errorf("assertion[2]: expected FileNotModified='c.go', got %q", assertions[2].FileNotModified)
	}
	if assertions[3].FileContains == nil || assertions[3].FileContains.Path != "d.go" || assertions[3].FileContains.Pattern != "hello" {
		t.Errorf("assertion[3]: unexpected FileContains: %+v", assertions[3].FileContains)
	}
	if assertions[4].FileNotContains == nil || assertions[4].FileNotContains.Path != "e.go" || assertions[4].FileNotContains.Pattern != "world" {
		t.Errorf("assertion[4]: unexpected FileNotContains: %+v", assertions[4].FileNotContains)
	}
}

func TestParseContractYAML_InvalidYAML(t *testing.T) {
	_, err := ParseContractYAML("not: [valid: yaml:")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseScenarios_MultipleScenarios(t *testing.T) {
	specMD := `# My Spec

## Scenarios

### Scenario: Add function works

**Given:** A calculator repo
**When:** The pipeline executes
**Then:**
- The Add function is implemented
- calc.go exists

### Scenario: Subtract function works

**Given:** A calculator repo
**When:** The pipeline executes
**Then:**
- The Subtract function is implemented
`
	scenarios, skipped, err := ParseScenarios(specMD)
	if err != nil {
		t.Fatalf("ParseScenarios: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("expected no skipped scenarios, got %v", skipped)
	}
	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d: %v", len(scenarios), scenarios)
	}
	if scenarios[0].Name != "Add function works" {
		t.Errorf("scenarios[0].Name = %q, want %q", scenarios[0].Name, "Add function works")
	}
	if scenarios[1].Name != "Subtract function works" {
		t.Errorf("scenarios[1].Name = %q, want %q", scenarios[1].Name, "Subtract function works")
	}
	if !strings.Contains(scenarios[0].When, "pipeline executes") {
		t.Errorf("scenarios[0].When = %q, expected to contain 'pipeline executes'", scenarios[0].When)
	}
	if !strings.Contains(scenarios[0].Then, "Add function is implemented") {
		t.Errorf("scenarios[0].Then = %q, expected to contain 'Add function is implemented'", scenarios[0].Then)
	}
	if !strings.Contains(scenarios[0].Given, "calculator repo") {
		t.Errorf("scenarios[0].Given = %q, expected to contain 'calculator repo'", scenarios[0].Given)
	}
}

func TestParseScenarios_MissingWhenSkipped(t *testing.T) {
	specMD := `# My Spec

## Scenarios

### Scenario: Has when and then

**When:** Something happens
**Then:** Something is true

### Scenario: Missing when

**Given:** Some context
**Then:** Something is true

### Scenario: Also valid

**When:** Another thing
**Then:** Another result
`
	scenarios, skipped, err := ParseScenarios(specMD)
	if err != nil {
		t.Fatalf("ParseScenarios: %v", err)
	}
	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios (one skipped for missing When), got %d: %v", len(scenarios), scenarios)
	}
	if scenarios[0].Name != "Has when and then" {
		t.Errorf("scenarios[0].Name = %q", scenarios[0].Name)
	}
	if scenarios[1].Name != "Also valid" {
		t.Errorf("scenarios[1].Name = %q", scenarios[1].Name)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped scenario, got %d: %v", len(skipped), skipped)
	}
	if !strings.Contains(skipped[0], "Missing when") {
		t.Errorf("skipped[0] = %q, expected to contain 'Missing when'", skipped[0])
	}
}

func TestParseScenarios_MissingThenSkipped(t *testing.T) {
	specMD := `# My Spec

## Scenarios

### Scenario: Has when and then

**When:** Something happens
**Then:** Something is true

### Scenario: Missing then

**Given:** Some context
**When:** Something happens
`
	scenarios, skipped, err := ParseScenarios(specMD)
	if err != nil {
		t.Fatalf("ParseScenarios: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario (one skipped for missing Then), got %d: %v", len(scenarios), scenarios)
	}
	if scenarios[0].Name != "Has when and then" {
		t.Errorf("scenarios[0].Name = %q", scenarios[0].Name)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped scenario, got %d: %v", len(skipped), skipped)
	}
	if !strings.Contains(skipped[0], "Missing then") {
		t.Errorf("skipped[0] = %q, expected to contain 'Missing then'", skipped[0])
	}
}

func TestParseScenarios_OptionalGivenAndNotes(t *testing.T) {
	specMD := `# My Spec

## Scenarios

### Scenario: No given no notes

**When:** Something happens
**Then:** Something is true

### Scenario: Has notes

**When:** Another thing
**Then:** Another result
**Notes:** This is a note about the scenario
`
	scenarios, _, err := ParseScenarios(specMD)
	if err != nil {
		t.Fatalf("ParseScenarios: %v", err)
	}
	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}
	if scenarios[0].Given != "" {
		t.Errorf("scenarios[0].Given should be empty, got %q", scenarios[0].Given)
	}
	if scenarios[0].Notes != "" {
		t.Errorf("scenarios[0].Notes should be empty, got %q", scenarios[0].Notes)
	}
	if !strings.Contains(scenarios[1].Notes, "note about the scenario") {
		t.Errorf("scenarios[1].Notes = %q, expected to contain 'note about the scenario'", scenarios[1].Notes)
	}
}

func TestParseScenarios_EmptyScenariosSection(t *testing.T) {
	specMD := `# My Spec

## Scenarios

## Next Section
`
	scenarios, _, err := ParseScenarios(specMD)
	if err != nil {
		t.Fatalf("ParseScenarios: %v", err)
	}
	if len(scenarios) != 0 {
		t.Errorf("expected 0 scenarios for empty section, got %d", len(scenarios))
	}
}

func TestParseScenarios_NoScenariosSection(t *testing.T) {
	specMD := `# My Spec

## Description
Some description here.

## Acceptance Criteria
- Something
`
	scenarios, _, err := ParseScenarios(specMD)
	if err != nil {
		t.Fatalf("ParseScenarios: %v", err)
	}
	if len(scenarios) != 0 {
		t.Errorf("expected 0 scenarios when no Scenarios section, got %d", len(scenarios))
	}
}
