package bead

import "testing"

func TestIsLowComplexityTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{
			name:  "matches migrate to pattern",
			title: "Migrate auth config to runtime settings",
			want:  true,
		},
		{
			name:  "matches wire into pattern",
			title: "Wire router into middleware stack",
			want:  true,
		},
		{
			name:  "matches add field pattern",
			title: "Add field retry_count to runner state",
			want:  true,
		},
		{
			name:  "matches add config pattern",
			title: "Add config timeout_override",
			want:  true,
		},
		{
			name:  "matches delete pattern",
			title: "Delete legacy route helper",
			want:  true,
		},
		{
			name:  "matches document pattern",
			title: "Document escalation flow",
			want:  true,
		},
		{
			name:  "matches rename pattern",
			title: "Rename tier selector helper",
			want:  true,
		},
		{
			name:  "matches add t parallel pattern",
			title: "Add t.Parallel to bead helper tests",
			want:  true,
		},
		{
			name:  "matches add compile time check pattern mixed case",
			title: "aDd CoMpIlE-TiMe ChEcK for interface compliance",
			want:  true,
		},
		{
			name:  "mixed case migrate still matches",
			title: "mIgRaTe parser state TO new config format",
			want:  true,
		},
		{
			name:  "non matching title",
			title: "Refactor tier selection for dynamic routing",
			want:  false,
		},
		{
			name:  "empty title",
			title: "   ",
			want:  false,
		},
		{
			name:  "partial word does not match migrate",
			title: "Migration plan for routing updates",
			want:  false,
		},
		{
			name:  "partial word does not match wire",
			title: "Rewire routing stack for plugin support",
			want:  false,
		},
		{
			name:  "partial word does not match rename",
			title: "Renamed helper methods for clarity",
			want:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsLowComplexityTitle(tt.title)
			if got != tt.want {
				t.Errorf("IsLowComplexityTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestIsLeafBead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		bead *Bead
		want bool
	}{
		{
			name: "nil bead",
			bead: nil,
			want: false,
		},
		{
			name: "nil dependent count is treated as leaf",
			bead: &Bead{},
			want: true,
		},
		{
			name: "zero dependent count is leaf",
			bead: &Bead{DependentCount: intPtr(0)},
			want: true,
		},
		{
			name: "positive dependent count is non-leaf",
			bead: &Bead{DependentCount: intPtr(2)},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsLeafBead(tt.bead)
			if got != tt.want {
				t.Errorf("IsLeafBead() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimatedFileCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		bead *Bead
		want int
	}{
		{
			name: "nil bead",
			bead: nil,
			want: 0,
		},
		{
			name: "no expected outputs",
			bead: &Bead{},
			want: 0,
		},
		{
			name: "nil expected outputs",
			bead: &Bead{ExpectedOutputs: nil},
			want: 0,
		},
		{
			name: "single expected output",
			bead: &Bead{ExpectedOutputs: []string{"internal/foo/bar.go"}},
			want: 1,
		},
		{
			name: "multiple expected outputs",
			bead: &Bead{ExpectedOutputs: []string{"a.go", "b.go", "c_test.go"}},
			want: 3,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := EstimatedFileCount(tt.bead)
			if got != tt.want {
				t.Errorf("EstimatedFileCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
}
