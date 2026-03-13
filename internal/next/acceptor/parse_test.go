package acceptor

import "testing"

func TestParseAcceptanceCriteria_BasicSection(t *testing.T) {
	specMD := `# My Spec

## Description
Some description here.

## Acceptance Criteria
- Refund endpoint returns 200
- Multi-currency support for USD and EUR
- Audit log entry created on refund

## Implementation Notes
Some notes.
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d: %v", len(criteria), criteria)
	}
	if criteria[0] != "Refund endpoint returns 200" {
		t.Errorf("criteria[0] = %q", criteria[0])
	}
}

func TestParseAcceptanceCriteria_NoSection(t *testing.T) {
	specMD := `# My Spec

## Description
No acceptance criteria section here.
`
	_, err := ParseAcceptanceCriteria(specMD)
	if err == nil {
		t.Error("expected error when no Acceptance Criteria section found")
	}
}

func TestParseAcceptanceCriteria_EmptySection(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria

## Next Section
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 0 {
		t.Errorf("expected 0 criteria for empty section, got %d", len(criteria))
	}
}

func TestParseAcceptanceCriteria_NumberedList(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria
1. First criterion
2. Second criterion
3. Third criterion

## Next Section
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d: %v", len(criteria), criteria)
	}
	if criteria[0] != "First criterion" {
		t.Errorf("criteria[0] = %q, want %q", criteria[0], "First criterion")
	}
	if criteria[1] != "Second criterion" {
		t.Errorf("criteria[1] = %q, want %q", criteria[1], "Second criterion")
	}
}

func TestParseAcceptanceCriteria_AsteriskBullets(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria
* First criterion
* Second criterion

## Next Section
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d: %v", len(criteria), criteria)
	}
	if criteria[0] != "First criterion" {
		t.Errorf("criteria[0] = %q, want %q", criteria[0], "First criterion")
	}
}

func TestParseAcceptanceCriteria_NestedBullets(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria
- Top-level criterion one
  - Nested detail A
  - Nested detail B
- Top-level criterion two
  - Nested detail C

## Next Section
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 2 {
		t.Fatalf("expected 2 top-level criteria (nested excluded), got %d: %v", len(criteria), criteria)
	}
	if criteria[0] != "Top-level criterion one" {
		t.Errorf("criteria[0] = %q", criteria[0])
	}
	if criteria[1] != "Top-level criterion two" {
		t.Errorf("criteria[1] = %q", criteria[1])
	}
}

func TestParseAcceptanceCriteria_RejectsPartialHeadingMatch(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria (Draft)
- Should not be extracted
- Also should not be extracted

## Notes
Some notes.
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err == nil {
		t.Fatalf("expected error for partial heading match, got criteria: %v", criteria)
	}
}

func TestParseAcceptanceCriteria_BoldPrefix(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria
- **Bold prefix** rest of text
- **Another bold** more text here

## Next Section
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d: %v", len(criteria), criteria)
	}
	if criteria[0] != "**Bold prefix** rest of text" {
		t.Errorf("criteria[0] = %q, want %q", criteria[0], "**Bold prefix** rest of text")
	}
}
