package coverage

import "testing"

func TestParseCriteria_ParsesBullets(t *testing.T) {
	spec := `# My Spec

Some intro.

## Acceptance Criteria

- Accept valid input
- Reject invalid input

## Next Section
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
	if criteria[0].Text != "Accept valid input" {
		t.Fatalf("criteria[0].Text = %q, want %q", criteria[0].Text, "Accept valid input")
	}

	if criteria[1].Number != 2 {
		t.Fatalf("criteria[1].Number = %d, want 2", criteria[1].Number)
	}
	if criteria[1].Text != "Reject invalid input" {
		t.Fatalf("criteria[1].Text = %q, want %q", criteria[1].Text, "Reject invalid input")
	}
}

func TestParseCriteria_CompoundCriteria(t *testing.T) {
	spec := `# My Spec

## Acceptance Criteria

- Validates input format
  including length, characters, and encoding
- Allows retries
`

	criteria, err := ParseCriteria(spec)
	if err != nil {
		t.Fatalf("ParseCriteria() error: %v", err)
	}

	if len(criteria) != 2 {
		t.Fatalf("len(criteria) = %d, want 2", len(criteria))
	}

	want := "Validates input format including length, characters, and encoding"
	if criteria[0].Text != want {
		t.Fatalf("criteria[0].Text = %q, want %q", criteria[0].Text, want)
	}
}
