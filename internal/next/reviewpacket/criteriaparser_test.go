package reviewpacket

import (
	"testing"
)

func TestParseAcceptanceCriteria_SingleCriterion(t *testing.T) {
	content := `
# My Spec

## Acceptance Criteria

1. Users can log in with valid credentials
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %d", len(criteria))
	}

	c := criteria[0]
	if c.Text != "Users can log in with valid credentials" {
		t.Errorf("expected text 'Users can log in with valid credentials', got '%s'", c.Text)
	}
}

func TestParseAcceptanceCriteria_MultipleCriteria(t *testing.T) {
	content := `
## Acceptance Criteria

1. First criterion
2. Second criterion
3. Third criterion
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d", len(criteria))
	}

	if criteria[0].Text != "First criterion" {
		t.Errorf("expected 'First criterion', got '%s'", criteria[0].Text)
	}
	if criteria[1].Text != "Second criterion" {
		t.Errorf("expected 'Second criterion', got '%s'", criteria[1].Text)
	}
	if criteria[2].Text != "Third criterion" {
		t.Errorf("expected 'Third criterion', got '%s'", criteria[2].Text)
	}
}

func TestParseAcceptanceCriteria_EmptyContent(t *testing.T) {
	content := ""
	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 0 {
		t.Fatalf("expected 0 criteria for empty content, got %d", len(criteria))
	}
}

func TestParseAcceptanceCriteria_NoCriteria(t *testing.T) {
	content := `
# Just a spec
## Some other section
But no acceptance criteria here.
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 0 {
		t.Fatalf("expected 0 criteria, got %d", len(criteria))
	}
}

func TestParseAcceptanceCriteria_WithWhitespace(t *testing.T) {
	content := `
## Acceptance Criteria

1.   Users can log in
2. System validates passwords
3.    Error messages are clear
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d", len(criteria))
	}

	if criteria[0].Text != "Users can log in" {
		t.Errorf("expected 'Users can log in', got '%s'", criteria[0].Text)
	}
	if criteria[1].Text != "System validates passwords" {
		t.Errorf("expected 'System validates passwords', got '%s'", criteria[1].Text)
	}
	if criteria[2].Text != "Error messages are clear" {
		t.Errorf("expected 'Error messages are clear', got '%s'", criteria[2].Text)
	}
}

func TestParseAcceptanceCriteria_StopsAtNextSection(t *testing.T) {
	content := `
## Acceptance Criteria

1. First criterion
2. Second criterion

## Other Section
Some content here
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(criteria))
	}
}

func TestParseAcceptanceCriteria_IgnoresNonNumberedLines(t *testing.T) {
	content := `
## Acceptance Criteria

1. Valid criterion
This line is not numbered
2. Another criterion
Not numbered either
3. Last criterion
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria (non-numbered lines ignored), got %d", len(criteria))
	}

	if criteria[0].Text != "Valid criterion" {
		t.Errorf("expected 'Valid criterion', got '%s'", criteria[0].Text)
	}
	if criteria[1].Text != "Another criterion" {
		t.Errorf("expected 'Another criterion', got '%s'", criteria[1].Text)
	}
	if criteria[2].Text != "Last criterion" {
		t.Errorf("expected 'Last criterion', got '%s'", criteria[2].Text)
	}
}

func TestParseAcceptanceCriteria_MultilineWithDashes(t *testing.T) {
	content := `
## Acceptance Criteria

1. First criterion with multiple parts:
   - Part A
   - Part B
2. Second criterion
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(criteria))
	}

	// Should capture just the first line of the criterion
	if criteria[0].Text != "First criterion with multiple parts:" {
		t.Errorf("expected 'First criterion with multiple parts:', got '%s'", criteria[0].Text)
	}
}

func TestParseAcceptanceCriteria_DashPrefixed(t *testing.T) {
	content := `
## Acceptance Criteria

- API returns paginated results
- Error handling for invalid inputs
- Performance meets SLA requirements
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d", len(criteria))
	}

	if criteria[0].Text != "API returns paginated results" {
		t.Errorf("expected 'API returns paginated results', got '%s'", criteria[0].Text)
	}
	if criteria[1].Text != "Error handling for invalid inputs" {
		t.Errorf("expected 'Error handling for invalid inputs', got '%s'", criteria[1].Text)
	}
	if criteria[2].Text != "Performance meets SLA requirements" {
		t.Errorf("expected 'Performance meets SLA requirements', got '%s'", criteria[2].Text)
	}
}

func TestParseAcceptanceCriteria_MixedNumberedAndDash(t *testing.T) {
	content := `
## Acceptance Criteria

1. First numbered criterion
- A dash-prefixed criterion
2. Second numbered criterion
`

	criteria := ParseAcceptanceCriteria(content)

	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d", len(criteria))
	}

	if criteria[0].Text != "First numbered criterion" {
		t.Errorf("expected 'First numbered criterion', got '%s'", criteria[0].Text)
	}
	if criteria[1].Text != "A dash-prefixed criterion" {
		t.Errorf("expected 'A dash-prefixed criterion', got '%s'", criteria[1].Text)
	}
	if criteria[2].Text != "Second numbered criterion" {
		t.Errorf("expected 'Second numbered criterion', got '%s'", criteria[2].Text)
	}
}
