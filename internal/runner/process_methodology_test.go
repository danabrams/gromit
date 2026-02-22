package runner

import (
	"testing"
)

func TestExtractRequirementsFromDescription_BulletedList(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "dash bullets",
			input: "- alpha\n- beta\n- gamma",
			want:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:  "asterisk bullets",
			input: "* one\n* two",
			want:  []string{"one", "two"},
		},
		{
			name:  "plus bullets",
			input: "+ first\n+ second",
			want:  []string{"first", "second"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRequirementsFromDescription(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d: %v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("item %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExtractRequirementsFromDescription_NumberedList(t *testing.T) {
	input := "Some preamble\n1. do this\n2. do that\n3. do the other thing"
	got := extractRequirementsFromDescription(input)
	want := []string{"do this", "do that", "do the other thing"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
