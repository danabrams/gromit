package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreshContextAdaptersRemoved(t *testing.T) {
	files := []string{
		"callbacks_tdd.go",
		"callbacks_tdd_test.go",
		"callbacks_tdd_missing_outputs_test.go",
		"tdd_pipeline_adapter.go",
		"tdd_pipeline_adapter_test.go",
	}
	for _, name := range files {
		path := filepath.Join(name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("unexpected fresh-context adapter %s still exists", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("checking %s: %v", path, err)
		}
	}
}
