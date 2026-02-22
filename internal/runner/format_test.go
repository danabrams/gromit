package runner

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{name: "zero", duration: 0, want: "0s"},
		{name: "seconds", duration: 30 * time.Second, want: "30s"},
		{name: "sub-second rounds up", duration: 500 * time.Millisecond, want: "1s"},
		{name: "sub-second rounds down", duration: 400 * time.Millisecond, want: "0s"},
		{name: "exactly one minute", duration: time.Minute, want: "1m"},
		{name: "minutes only", duration: 5 * time.Minute, want: "5m"},
		{name: "hours and minutes", duration: 2*time.Hour + 15*time.Minute, want: "2h 15m"},
		{name: "hours with zero minutes", duration: 1 * time.Hour, want: "1h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}
