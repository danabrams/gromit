package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBaselineLogPath_FormatsTimestampAndPath(t *testing.T) {
	now := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.Local)

	got := baselineLogPath(now)
	want := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506.log")

	if got != want {
		t.Fatalf("baselineLogPath() = %q, want %q", got, want)
	}
}
