package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTDDPackageRemoved(t *testing.T) {
	path := filepath.Join("tdd")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s still exists; delete it as specified by the removal task", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error when checking %s: %v", path, err)
	}
}
