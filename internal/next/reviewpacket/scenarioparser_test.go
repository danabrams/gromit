package reviewpacket

import (
	"strings"
	"testing"
)

func TestParseScenarios_SingleScenario(t *testing.T) {
	content := `
# My Spec

## Behavioral Scenarios

### Scenario: User logs in successfully
Given a registered user with email "test@example.com"
When the user submits valid credentials
Then the user is redirected to the dashboard
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	s := scenarios[0]
	if s.Title != "User logs in successfully" {
		t.Errorf("expected title 'User logs in successfully', got '%s'", s.Title)
	}
	if s.Given != `a registered user with email "test@example.com"` {
		t.Errorf("expected Given, got '%s'", s.Given)
	}
	if s.When != "the user submits valid credentials" {
		t.Errorf("expected When, got '%s'", s.When)
	}
	if s.Then != "the user is redirected to the dashboard" {
		t.Errorf("expected Then, got '%s'", s.Then)
	}
}

func TestParseScenarios_MultipleScenarios(t *testing.T) {
	content := `
### Scenario: Happy path
Given initial state A
When action B
Then result C

### Scenario: Error case
Given invalid data
When processing occurs
Then error is returned
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}

	if scenarios[0].Title != "Happy path" {
		t.Errorf("expected first scenario title 'Happy path', got '%s'", scenarios[0].Title)
	}
	if scenarios[1].Title != "Error case" {
		t.Errorf("expected second scenario title 'Error case', got '%s'", scenarios[1].Title)
	}
}

func TestParseScenarios_EmptyContent(t *testing.T) {
	content := ""
	scenarios := ParseScenarios(content)

	if len(scenarios) != 0 {
		t.Fatalf("expected 0 scenarios for empty content, got %d", len(scenarios))
	}
}

func TestParseScenarios_NoScenarios(t *testing.T) {
	content := `
# Just a spec
## With some sections
But no scenarios here.
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 0 {
		t.Fatalf("expected 0 scenarios, got %d", len(scenarios))
	}
}

func TestParseScenarios_GivenWhenThenOnlyLines(t *testing.T) {
	content := `
### Scenario: Test scenario
Given something
When something happens
Then result happens
Extra text here that shouldn't be captured
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	s := scenarios[0]
	if s.Given != "something" {
		t.Errorf("Given should be 'something', got '%s'", s.Given)
	}
	if s.When != "something happens" {
		t.Errorf("When should be 'something happens', got '%s'", s.When)
	}
	if s.Then != "result happens" {
		t.Errorf("Then should be 'result happens', got '%s'", s.Then)
	}
}

func TestParseScenarios_MissingGivenWhenThen(t *testing.T) {
	content := `
### Scenario: Incomplete scenario
Given something
When something happens
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	s := scenarios[0]
	if s.Given != "something" {
		t.Errorf("Given should be 'something', got '%s'", s.Given)
	}
	if s.When != "something happens" {
		t.Errorf("When should be 'something happens', got '%s'", s.When)
	}
	if s.Then != "" {
		t.Errorf("Then should be empty, got '%s'", s.Then)
	}
}

func TestParseScenarios_CaseInsensitiveKeywords(t *testing.T) {
	content := `
### Scenario: Test case sensitivity
GIVEN uppercase given
WHEN uppercase when
THEN uppercase then
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	s := scenarios[0]
	if s.Given != "uppercase given" {
		t.Errorf("Given should match despite case, got '%s'", s.Given)
	}
	if s.When != "uppercase when" {
		t.Errorf("When should match despite case, got '%s'", s.When)
	}
	if s.Then != "uppercase then" {
		t.Errorf("Then should match despite case, got '%s'", s.Then)
	}
}

func TestParseScenarios_WhitespaceHandling(t *testing.T) {
	content := `
### Scenario: Whitespace test
Given   extra    spaces
When	tab	separated
Then  leading and trailing spaces
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	s := scenarios[0]
	// Should trim leading/trailing whitespace and collapse internal spaces
	if !strings.Contains(s.Given, "extra") || !strings.Contains(s.Given, "spaces") {
		t.Errorf("Given should contain 'extra' and 'spaces', got '%s'", s.Given)
	}
}

func TestParseScenarios_MultilineScenario(t *testing.T) {
	content := `
### Scenario: Complex scenario
Given a user is logged in
  and has created multiple items
  and items are in the draft state
When the user navigates to the archive
  and selects all items
Then all items are archived
  and the user sees a confirmation message
`

	scenarios := ParseScenarios(content)

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	s := scenarios[0]
	if s.Title != "Complex scenario" {
		t.Errorf("expected title 'Complex scenario', got '%s'", s.Title)
	}
	// Each field should capture just the first line, not continuation
	if s.Given != "a user is logged in" {
		t.Errorf("Given should be first line only, got '%s'", s.Given)
	}
	if s.When != "the user navigates to the archive" {
		t.Errorf("When should be first line only, got '%s'", s.When)
	}
	if s.Then != "all items are archived" {
		t.Errorf("Then should be first line only, got '%s'", s.Then)
	}
}
