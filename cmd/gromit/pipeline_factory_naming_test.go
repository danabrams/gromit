package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIPipelineFactoryNaming(t *testing.T) {
	banned := []string{
		"boardPipelineFactory",
		"queuePipelineFactory",
		"createDecomposePipeline(",
		"createDecomposePipeline =",
		"createReviewPipeline(",
		"createReviewPipeline =",
	}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if filepath.Base(path) == "pipeline_factory_naming_test.go" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, term := range banned {
			if strings.Contains(string(content), term) {
				t.Fatalf("found banned pipeline factory name %q in %s", term, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking cmd/gromit: %v", err)
	}
}
