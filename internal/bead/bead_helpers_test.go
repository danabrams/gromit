package bead

import "testing"

func TestIsLeafBead(t *testing.T) {
	t.Parallel()

	two := 2
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
			bead: &Bead{DependentCount: &two},
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
