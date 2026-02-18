package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBaselineLogPath_FormatsTimestampAndPath(t *testing.T) {
	now := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.Local)

	originalExists := baselineLogPathExistsFn
	baselineLogPathExistsFn = func(path string) (bool, error) {
		return false, nil
	}
	t.Cleanup(func() {
		baselineLogPathExistsFn = originalExists
	})

	got, err := baselineLogPath(now)
	if err != nil {
		t.Fatalf("baselineLogPath() error = %v", err)
	}
	want := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506.log")

	if got != want {
		t.Fatalf("baselineLogPath() = %q, want %q", got, want)
	}
}

func TestBaselineLogPath_ReturnsCanonicalWhenNoCollision(t *testing.T) {
	now := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.Local)
	want := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506.log")

	originalExists := baselineLogPathExistsFn
	var sawPath string
	baselineLogPathExistsFn = func(path string) (bool, error) {
		sawPath = path
		return false, nil
	}
	t.Cleanup(func() {
		baselineLogPathExistsFn = originalExists
	})

	got, err := baselineLogPath(now)
	if err != nil {
		t.Fatalf("baselineLogPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("baselineLogPath() = %q, want %q", got, want)
	}
	if sawPath != want {
		t.Fatalf("baselineLogPath() existence check = %q, want %q", sawPath, want)
	}
}

func TestBaselineLogPathNow_UsesCurrentLocalTime(t *testing.T) {
	stub := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.Local)

	originalNow := baselineLogPathNowFn
	baselineLogPathNowFn = func() time.Time {
		return stub
	}
	t.Cleanup(func() {
		baselineLogPathNowFn = originalNow
	})

	originalExists := baselineLogPathExistsFn
	baselineLogPathExistsFn = func(path string) (bool, error) {
		return false, nil
	}
	t.Cleanup(func() {
		baselineLogPathExistsFn = originalExists
	})

	got, err := baselineLogPathNow()
	if err != nil {
		t.Fatalf("baselineLogPathNow() error = %v", err)
	}
	want := filepath.Join("test-logs", "refactor-baseline-2025-01-02-030405.log")

	if got != want {
		t.Fatalf("baselineLogPathNow() = %q, want %q", got, want)
	}
}
