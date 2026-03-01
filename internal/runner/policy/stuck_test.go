package policy_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func TestNewConfigStuckPolicy_NotNil(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	p := policy.NewConfigStuckPolicy(cfg)

	if p == nil {
		t.Fatal("expected NewConfigStuckPolicy to return a policy")
	}
}

func TestNewConfigStuckPolicy_UsesThreshold(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Loop: config.LoopConfig{
			StuckBeadThreshold: 3,
		},
	}

	p := policy.NewConfigStuckPolicy(cfg)
	b := &bead.Bead{ID: "bead-1"}

	cases := []struct {
		name     string
		failures int
		want     bool
	}{
		{"below threshold", 2, false},
		{"at threshold", 3, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stats := map[string]logger.BeadStats{
				b.ID: {Failures: tc.failures},
			}
			if got := p.IsStuck(b, stats); got != tc.want {
				t.Errorf("IsStuck() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestThresholdStuckPolicy_IsStuck_Boundary(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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

func TestThresholdStuckPolicy_IsStuck_MissingStats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		stats map[string]logger.BeadStats
	}{
		{"nil stats", nil},
		{"missing bead entry", map[string]logger.BeadStats{}},
	}

	b := &bead.Bead{ID: "bead-1"}
	p := policy.NewThresholdStuckPolicy(3)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := p.IsStuck(b, tc.stats); got {
				t.Errorf("IsStuck() = true, want false")
			}
		})
	}
}

func TestThresholdStuckPolicy_IsStuck_AlternatingMultiBeadCycles(t *testing.T) {
	t.Parallel()

	// In an alternating multi-bead cycle, a bead with high failure rate
	// should be marked stuck, but one with moderate failure rate should not.
	cases := []struct {
		name     string
		failures int
		total    int
		threshold int
		want     bool
	}{
		{"100% failure rate (all attempts failed)", 3, 3, 2, true},
		{"100% failure rate with high threshold", 2, 2, 1, true},
		{"partial failure (low rate)", 1, 5, 2, false},
		{"exactly 50% failure rate", 2, 4, 2, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := &bead.Bead{ID: "multi-cycle-bead"}
			p := policy.NewThresholdStuckPolicy(tc.threshold)
			stats := map[string]logger.BeadStats{
				b.ID: {
					Failures:  tc.failures,
					TotalRuns: tc.total,
				},
			}
			if got := p.IsStuck(b, stats); got != tc.want {
				t.Errorf("IsStuck() = %v, want %v", got, tc.want)
			}
		})
	}
}
