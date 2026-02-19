package coverage

import (
	"testing"
)

func TestParseCriteria_ExtractsBullets(t *testing.T) {
	spec := `# My Spec

Some description here.

## Acceptance Criteria

- AC1: The system accepts valid input
- AC2: The system rejects invalid input
`

	criteria, err := ParseCriteria(spec)
	if err != nil {
		t.Fatalf("ParseCriteria() error: %v", err)
	}

	if len(criteria) != 2 {
		t.Fatalf("len(criteria) = %d, want 2", len(criteria))
	}

	if criteria[0].Number != 1 {
		t.Fatalf("criteria[0].Number = %d, want 1", criteria[0].Number)
	}
	if criteria[0].Text != "The system accepts valid input" {
		t.Fatalf("criteria[0].Text = %q, want %q", criteria[0].Text, "The system accepts valid input")
	}

	if criteria[1].Number != 2 {
		t.Fatalf("criteria[1].Number = %d, want 2", criteria[1].Number)
	}
	if criteria[1].Text != "The system rejects invalid input" {
		t.Fatalf("criteria[1].Text = %q, want %q", criteria[1].Text, "The system rejects invalid input")
	}
}
