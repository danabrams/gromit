//go:build acceptance

package acceptance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

func captureStatusOutputs(t *testing.T, gromitDir string, cfg *config.Config, runs int) []string {
	t.Helper()
	if runs <= 0 {
		t.Fatalf("runs must be positive, got %d", runs)
	}

	outputs := make([]string, 0, runs)
	for i := 0; i < runs; i++ {
		var buf strings.Builder
		if err := runner.PrintStatus(gromitDir, cfg, &buf, nil, false); err != nil {
			t.Fatalf("PrintStatus call %d failed: %v", i+1, err)
		}
		outputs = append(outputs, buf.String())
	}
	return outputs
}

func readIntegrationQueueContents(t *testing.T, gromitDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(gromitDir, "integration-queue.json"))
	if err != nil {
		t.Fatalf("reading integration queue file: %v", err)
	}
	return string(data)
}
