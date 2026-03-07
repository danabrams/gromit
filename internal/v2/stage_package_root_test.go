package v2

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestStagePackageRoot(t *testing.T) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to determine test file path")
	}
	want := filepath.Join(filepath.Dir(file), "stage")
	if got := stagePackageRoot(); got != want {
		t.Fatalf("expected stage package root %s, got %s", want, got)
	}
}
