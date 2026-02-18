package policy_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func TestThresholdStuckPolicy_IsStuck_Boundary(t *testing.T) {
	cases := []struct {
		name      string
		failures  int
		threshold int
		want      bool
	}{
		{"below threshold", 2, 3, false},
		{"at threshold", 3, 3, true},
	}

	b := &bead.Bead{ID: "bead-1"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := policy.NewThresholdStuckPolicy(tc.threshold)
			stats := map[string]logger.BeadStats{
				b.ID: {Failures: tc.failures},
			}
			if got := p.IsStuck(b, stats); got != tc.want {
				t.Errorf("IsStuck() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestThresholdStuckPolicy_IsStuck_DisabledThreshold(t *testing.T) {
	cases := []struct {
		name      string
		threshold int
	}{
		{"zero threshold", 0},
		{"negative threshold", -1},
	}

	b := &bead.Bead{ID: "bead-1"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := policy.NewThresholdStuckPolicy(tc.threshold)
			stats := map[string]logger.BeadStats{
				b.ID: {Failures: 10},
			}
			if got := p.IsStuck(b, stats); got {
				t.Errorf("IsStuck() = true, want false")
			}
		})
	}
}
