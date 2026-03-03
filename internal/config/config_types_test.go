package config

import (
	"testing"
	"time"
)

func TestConfigMidBuildReviewTimeout(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	cfg.MidBuildReview.Timeout = DurationSeconds(30 * time.Second)

	if got := cfg.MidBuildReviewTimeout(); got != 30*time.Second {
		t.Fatalf("MidBuildReviewTimeout() = %v, want %v", got, 30*time.Second)
	}
}
