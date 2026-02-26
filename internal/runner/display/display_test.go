package display

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRun_nilStatus(t *testing.T) {
	t.Parallel()

	got := FormatRun(nil)
	if !strings.Contains(got, "Run: not running") {
		t.Fatalf("FormatRun(nil) = %q, want substring %q", got, "Run: not running")
	}
}

func TestFormatHealth_defaults(t *testing.T) {
	t.Parallel()

	got := FormatHealth(time.Time{}, 0)
	if !strings.Contains(got, "Health:") {
		t.Fatalf("FormatHealth() = %q, want substring %q", got, "Health:")
	}
}
