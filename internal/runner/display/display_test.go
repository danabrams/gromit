package display

import (
    "strings"
    "testing"
)

func TestFormatRun_nilStatus(t *testing.T) {
    t.Parallel()

    got := FormatRun(nil)
    if !strings.Contains(got, "Run: not running") {
        t.Fatalf("FormatRun(nil) = %q, want substring %q", got, "Run: not running")
    }
}
