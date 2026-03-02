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
		name      string
		failures  int
		total     int
		threshold int
		want      bool
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

func TestThresholdStuckPolicy_IsStuck_LegacyDataWithoutTotalRuns(t *testing.T) {
	t.Parallel()

	// Backward compatibility: when TotalRuns is 0 (missing data), fall back to threshold check.
	b := &bead.Bead{ID: "legacy-bead"}
	p := policy.NewThresholdStuckPolicy(2)

	stats := map[string]logger.BeadStats{
		b.ID: {
			Failures:  3,
			TotalRuns: 0, // No run data recorded (legacy entry or corrupted)
		},
	}

	// Should still mark stuck when failures >= threshold, even with missing TotalRuns
	if got := p.IsStuck(b, stats); !got {
		t.Errorf("IsStuck() = false, want true (backward compatibility for legacy data)")
	}
}

func TestThresholdStuckPolicy_IsStuck_ThresholdMeetsButPartialFailure(t *testing.T) {
	t.Parallel()

	// RED test: A bead at exactly the threshold failure count but with partial success
	// should NOT be marked stuck in a multi-bead cycle scenario.
	b := &bead.Bead{ID: "partial-success-bead"}
	p := policy.NewThresholdStuckPolicy(3)

	stats := map[string]logger.BeadStats{
		b.ID: {
			Failures:  3,
			TotalRuns: 5, // 3 failures out of 5 runs = 60% failure rate
		},
	}

	// Even though failures >= threshold (3 >= 3), not all attempts failed,
	// so it should NOT be marked stuck
	if got := p.IsStuck(b, stats); got {
		t.Errorf("IsStuck() = true, want false (bead has some successes)")
	}
}
