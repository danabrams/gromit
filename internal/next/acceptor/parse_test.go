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
