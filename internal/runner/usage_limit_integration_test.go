//go:build acceptance || integration

package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/usagelimit"
)

// TestUsageLimitDetection_Integration verifies that when a Claude result has
// failure signals matching usage limit patterns, the detection logic correctly
// identifies it.
func TestUsageLimitDetection_Integration(t *testing.T) {
	tests := []struct {
		name          string
		result        *claude.Result
		stats         *logger.StreamStats
		wantDetected  bool
		wantErrSubstr string
	}{
		{
			name: "keyword match in output",
			result: &claude.Result{
				Success:  false,
				Output:   "Error: usage limit exceeded for your plan",
				ExitCode: 1,
			},
			stats:         &logger.StreamStats{RateLimitHits: 0},
			wantDetected:  true,
			wantErrSubstr: "usage limit",
		},
		{
			name: "rate limit hits with failed invocation",
			result: &claude.Result{
				Success:  false,
				Output:   "generic error",
				ExitCode: 1,
			},
			stats:         &logger.StreamStats{RateLimitHits: 5},
			wantDetected:  true,
			wantErrSubstr: "usage limit",
		},
		{
			name: "normal build failure",
			result: &claude.Result{
				Success:  false,
				Output:   "tests failed",
				ExitCode: 1,
			},
			stats:        &logger.StreamStats{RateLimitHits: 0},
			wantDetected: false,
		},
		{
			name: "success with rate limit hits does not trigger",
			result: &claude.Result{
				Success:  true,
				Output:   "completed successfully",
				ExitCode: 0,
			},
			stats:        &logger.StreamStats{RateLimitHits: 3},
			wantDetected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct signals from result and stats
			signals := usagelimit.Signals{
				ExitCode:      tt.result.ExitCode,
				Output:        tt.result.Output,
				RateLimitHits: 0,
			}
			if tt.stats != nil {
				signals.RateLimitHits = tt.stats.RateLimitHits
			}

			// Check detection
			patterns := usagelimit.ClaudePatterns()
			detected := usagelimit.Check(signals, patterns)

			if detected != tt.wantDetected {
				t.Errorf("expected detected=%v, got %v", tt.wantDetected, detected)
			}
		})
	}
}
