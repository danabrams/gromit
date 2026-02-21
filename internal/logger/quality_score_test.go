package logger

import (
	"math"
	"testing"
)

func TestComputeQualityScore(t *testing.T) {
	tests := []struct {
		name              string
		criteriaTotal     int
		criteriaCovered   int
		validationRetried bool
		trivialAutoFixed  bool
		escalated         bool
		reviewFixes       int
		want              float64
	}{
		{
			name:            "perfect criteria no penalties",
			criteriaTotal:   5,
			criteriaCovered: 5,
			want:            1.0,
		},
		{
			name:              "coverage and penalties",
			criteriaTotal:     10,
			criteriaCovered:   8,
			validationRetried: true,
			escalated:         true,
			reviewFixes:       2,
			want:              0.45, // 0.8 - (0.1 + 0.15 + 0.1)
		},
		{
			name:             "no criteria defaults to one",
			trivialAutoFixed: true,
			want:             0.9,
		},
		{
			name:              "clamped to zero",
			criteriaTotal:     2,
			criteriaCovered:   0,
			validationRetried: true,
			trivialAutoFixed:  true,
			escalated:         true,
			reviewFixes:       10,
			want:              0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeQualityScore(tc.criteriaTotal, tc.criteriaCovered, tc.validationRetried, tc.trivialAutoFixed, tc.escalated, tc.reviewFixes)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("ComputeQualityScore() = %v, want %v", got, tc.want)
			}
		})
	}
}
