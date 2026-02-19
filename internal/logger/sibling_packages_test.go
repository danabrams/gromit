package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadSiblingTouchedPackages_UnionBySpecOrParentCompletedOnly(t *testing.T) {
	dir := t.TempDir()
	writeIterationFixture(t, dir, "run-20260219-100000.jsonl", []IterationLog{
		{BeadID: "self", Success: true, SpecID: "spec-A", TouchedPackages: []string{"internal/self"}},
		{BeadID: "sibling-spec-1", Success: true, SpecID: "spec-A", TouchedPackages: []string{"internal/speca", "internal/shared"}},
		{BeadID: "sibling-spec-2", Success: false, SpecID: "spec-A", TouchedPackages: []string{"internal/failed"}},
		{BeadID: "other-spec", Success: true, SpecID: "spec-B", TouchedPackages: []string{"internal/other"}},
	})
	writeIterationFixture(t, dir, "run-20260219-110000.jsonl", []IterationLog{
		{BeadID: "sibling-parent-1", Success: true, SpecID: "spec-Z", TouchedPackages: []string{"internal/parent", "internal/shared"}},
		{BeadID: "sibling-parent-2", Success: false, SpecID: "spec-Z", TouchedPackages: []string{"internal/parent-failed"}},
		{BeadID: "unrelated", Success: true, SpecID: "spec-Z", TouchedPackages: []string{"internal/unrelated"}},
	})

	got, err := ReadSiblingTouchedPackages(dir, "self", "spec-A", []string{"sibling-parent-1", "sibling-parent-2"})
	if err != nil {
		t.Fatalf("ReadSiblingTouchedPackages() error = %v", err)
	}

	want := []string{"internal/parent", "internal/shared", "internal/speca"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadSiblingTouchedPackages() = %v, want %v", got, want)
	}
}

func TestReadSiblingTouchedPackagesBySpec_IncludesOnlyCompletedMatchingSpec(t *testing.T) {
	dir := t.TempDir()
	writeIterationFixture(t, dir, "run-20260219-120000.jsonl", []IterationLog{
		{BeadID: "self", Success: true, SpecID: "spec-A", TouchedPackages: []string{"internal/self"}},
		{BeadID: "b1", Success: true, SpecID: "spec-A", TouchedPackages: []string{"internal/a", "internal/b"}},
		{BeadID: "b2", Success: false, SpecID: "spec-A", TouchedPackages: []string{"internal/fail"}},
		{BeadID: "b3", Success: true, SpecID: "spec-B", TouchedPackages: []string{"internal/other"}},
	})

	got, err := ReadSiblingTouchedPackagesBySpec(dir, "self", "spec-A")
	if err != nil {
		t.Fatalf("ReadSiblingTouchedPackagesBySpec() error = %v", err)
	}

	want := []string{"internal/a", "internal/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadSiblingTouchedPackagesBySpec() = %v, want %v", got, want)
	}
}

func TestReadSiblingTouchedPackagesByParent_IncludesOnlyCompletedProvidedSiblings(t *testing.T) {
	dir := t.TempDir()
	writeIterationFixture(t, dir, "run-20260219-130000.jsonl", []IterationLog{
		{BeadID: "self", Success: true, SpecID: "spec-A", TouchedPackages: []string{"internal/self"}},
		{BeadID: "sib-1", Success: true, SpecID: "spec-X", TouchedPackages: []string{"internal/one"}},
		{BeadID: "sib-2", Success: false, SpecID: "spec-X", TouchedPackages: []string{"internal/two"}},
		{BeadID: "other", Success: true, SpecID: "spec-X", TouchedPackages: []string{"internal/other"}},
	})

	got, err := ReadSiblingTouchedPackagesByParent(dir, "self", []string{"sib-1", "sib-2"})
	if err != nil {
		t.Fatalf("ReadSiblingTouchedPackagesByParent() error = %v", err)
	}

	want := []string{"internal/one"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadSiblingTouchedPackagesByParent() = %v, want %v", got, want)
	}
}

func TestReadSiblingTouchedPackages_NoContextReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	writeIterationFixture(t, dir, "run-20260219-140000.jsonl", []IterationLog{
		{BeadID: "b1", Success: true, SpecID: "spec-A", TouchedPackages: []string{"internal/a"}},
	})

	got, err := ReadSiblingTouchedPackages(dir, "self", "", nil)
	if err != nil {
		t.Fatalf("ReadSiblingTouchedPackages() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadSiblingTouchedPackages() = %v, want empty", got)
	}
	if got == nil {
		t.Fatal("ReadSiblingTouchedPackages() returned nil slice, want empty slice")
	}
}

func writeIterationFixture(t *testing.T, dir, fileName string, entries []IterationLog) {
	t.Helper()

	path := filepath.Join(dir, fileName)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating fixture file %s: %v", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("closing fixture file %s: %v", path, closeErr)
		}
	}()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		entry.Iteration = 1
		entry.Model = "sonnet"
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encoding fixture entry: %v", err)
		}
	}
}
