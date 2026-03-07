package generation

import "testing"

func TestParseGenerationLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		label string
		want  int
		ok    bool
	}{
		{"valid", "gen:3", 3, true},
		{"zero", "gen:0", 0, true},
		{"missing value", "gen:", 0, false},
		{"invalid prefix", "generation:1", 0, false},
		{"non numeric", "gen:abc", 0, false},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseGenerationLabel(tt.label)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseGenerationLabel(%q) = (%d, %t), want (%d, %t)", tt.label, got, ok, tt.want, tt.ok)
			}
		})
	}
}
