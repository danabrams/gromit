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

func TestBaselineLogPath_AppendsSuffixOnCollision(t *testing.T) {
	now := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.Local)
	base := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506.log")
	want := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506-1.log")

	originalExists := baselineLogPathExistsFn
	var sawPaths []string
	baselineLogPathExistsFn = func(path string) (bool, error) {
		sawPaths = append(sawPaths, path)
		if path == base {
			return true, nil
		}
		if path == want {
			return false, nil
		}
		t.Fatalf("unexpected path check: %q", path)
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
	if len(sawPaths) != 2 {
		t.Fatalf("baselineLogPath() checked %d paths, want 2", len(sawPaths))
	}
	if sawPaths[0] != base {
		t.Fatalf("baselineLogPath() first check = %q, want %q", sawPaths[0], base)
	}
	if sawPaths[1] != want {
		t.Fatalf("baselineLogPath() second check = %q, want %q", sawPaths[1], want)
	}
}

func TestBaselineLogPath_ErrorsOnRepeatedCollision(t *testing.T) {
	now := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.Local)
	base := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506.log")
	suffix := filepath.Join("test-logs", "refactor-baseline-2026-02-03-040506-1.log")

	originalExists := baselineLogPathExistsFn
	baselineLogPathExistsFn = func(path string) (bool, error) {
		if path == base || path == suffix {
			return true, nil
		}
		t.Fatalf("unexpected path check: %q", path)
		return false, nil
	}
	t.Cleanup(func() {
		baselineLogPathExistsFn = originalExists
	})

	_, err := baselineLogPath(now)
	if err == nil {
		t.Fatal("baselineLogPath() error = nil, want collision error")
	}
	if err != errBaselineLogPathCollision {
		t.Fatalf("baselineLogPath() error = %v, want %v", err, errBaselineLogPathCollision)
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

func TestInitBaselineLogPathForRun_UsesResolvedPathInConsumer(t *testing.T) {
	stub := time.Date(2026, time.February, 4, 7, 8, 9, 0, time.Local)
	want := filepath.Join("test-logs", "refactor-baseline-2026-02-04-070809.log")

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

	var consumed string
	consumeFn := func(path string) error {
		consumed = path
		return nil
	}

	got, err := initBaselineLogPathForRun(consumeFn)
	if err != nil {
		t.Fatalf("initBaselineLogPathForRun() error = %v", err)
	}
	if got != want {
		t.Fatalf("initBaselineLogPathForRun() = %q, want %q", got, want)
	}
	if consumed != got {
		t.Fatalf("initBaselineLogPathForRun() consumed %q, want %q", consumed, got)
	}
}
