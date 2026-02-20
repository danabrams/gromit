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

	criteria := mustParseCriteria(t, spec)
	assertCriterion(t, criteria, 0, 1, "Accept valid input")
	assertCriterion(t, criteria, 1, 2, "Reject invalid input")
}

func TestParseCriteria_CompoundCriteria(t *testing.T) {
	spec := `# My Spec

## Acceptance Criteria

- Validates input format; handles commas, semicolons, and periods
  including length, characters, and encoding
- Allows retries
`

	criteria := mustParseCriteria(t, spec)
	if len(criteria) != 2 {
		t.Fatalf("len(criteria) = %d, want 2", len(criteria))
	}

	want := "Validates input format; handles commas, semicolons, and periods including length, characters, and encoding"
	if criteria[0].Text != want {
		t.Fatalf("criteria[0].Text = %q, want %q", criteria[0].Text, want)
	}
}

func TestParseCriteria_MissingSection(t *testing.T) {
	spec := `# My Spec

No acceptance criteria here.
`

	criteria := mustParseCriteria(t, spec)

	if len(criteria) != 0 {
		t.Fatalf("len(criteria) = %d, want 0", len(criteria))
	}
}

func TestParseCriteria_ParsesListFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
		want []string
	}{
		{
			name: "asterisk bullets",
			spec: `# My Spec

## Acceptance Criteria

* Supports import mode
* Supports export mode
`,
			want: []string{"Supports import mode", "Supports export mode"},
		},
		{
			name: "numbered list",
			spec: `# My Spec

## Acceptance Criteria

1. Creates report files
2) Uploads report files
`,
			want: []string{"Creates report files", "Uploads report files"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			criteria := mustParseCriteria(t, tt.spec)
			if len(criteria) != len(tt.want) {
				t.Fatalf("len(criteria) = %d, want %d", len(criteria), len(tt.want))
			}
			for i, want := range tt.want {
				assertCriterion(t, criteria, i, i+1, want)
			}
		})
	}
}

func mustParseCriteria(t *testing.T, spec string) []Criterion {
	t.Helper()

	criteria, err := ParseCriteria(spec)
	if err != nil {
		t.Fatalf("ParseCriteria() error: %v", err)
	}
	return criteria
}

func assertCriterion(t *testing.T, criteria []Criterion, index int, wantNumber int, wantText string) {
	t.Helper()

	if len(criteria) <= index {
		t.Fatalf("criteria[%d] missing; len(criteria) = %d", index, len(criteria))
	}
	if criteria[index].Number != wantNumber {
		t.Fatalf("criteria[%d].Number = %d, want %d", index, criteria[index].Number, wantNumber)
	}
	if criteria[index].Text != wantText {
		t.Fatalf("criteria[%d].Text = %q, want %q", index, criteria[index].Text, wantText)
	}
}
