package procutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPIDPressureNormal(t *testing.T) {
	dir := t.TempDir()
	curPath := filepath.Join(dir, "pids.current")
	maxPath := filepath.Join(dir, "pids.max")

	os.WriteFile(curPath, []byte("500\n"), 0o644)
	os.WriteFile(maxPath, []byte("8137\n"), 0o644)

	cur, max, err := pidPressureFrom(curPath, maxPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cur != 500 {
		t.Errorf("current = %d, want 500", cur)
	}
	if max != 8137 {
		t.Errorf("max = %d, want 8137", max)
	}
}

func TestPIDPressureUnlimited(t *testing.T) {
	dir := t.TempDir()
	curPath := filepath.Join(dir, "pids.current")
	maxPath := filepath.Join(dir, "pids.max")

	os.WriteFile(curPath, []byte("42\n"), 0o644)
	os.WriteFile(maxPath, []byte("max\n"), 0o644)

	cur, max, err := pidPressureFrom(curPath, maxPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cur != 42 {
		t.Errorf("current = %d, want 42", cur)
	}
	if max != 0 {
		t.Errorf("max = %d, want 0 (unlimited)", max)
	}
}

func TestPIDPressureMissingFiles(t *testing.T) {
	dir := t.TempDir()
	curPath := filepath.Join(dir, "pids.current")
	maxPath := filepath.Join(dir, "pids.max")

	// Neither file exists.
	_, _, err := pidPressureFrom(curPath, maxPath)
	if err == nil {
		t.Fatal("expected error for missing files, got nil")
	}

	// Only current exists — max is missing.
	os.WriteFile(curPath, []byte("10\n"), 0o644)
	_, _, err = pidPressureFrom(curPath, maxPath)
	if err == nil {
		t.Fatal("expected error for missing pids.max, got nil")
	}
}
