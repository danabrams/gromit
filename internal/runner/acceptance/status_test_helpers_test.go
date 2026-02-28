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

func assertIntegrationQueueSectionStable(t *testing.T, outputs []string) {
	t.Helper()
	if len(outputs) < 2 {
		t.Fatalf("expected at least two outputs, got %d", len(outputs))
	}
	ref := extractIntegrationQueueSection(outputs[0])
	if ref == "" {
		t.Fatalf("first output missing Integration Queue section\n%s", outputs[0])
	}
	for i := 1; i < len(outputs); i++ {
		candidate := extractIntegrationQueueSection(outputs[i])
		if candidate != ref {
			t.Fatalf("Integration Queue section mutated between runs\nexpected:\n%s\nactual (call %d):\n%s", ref, i+1, candidate)
		}
	}
}

func extractIntegrationQueueSection(output string) string {
	header := "Integration Queue:"
	start := strings.Index(output, header)
	if start == -1 {
		return ""
	}
	section := output[start:]
	markers := []string{"Next action:", "Health:", "Model Performance:", "SPC:"}
	end := len(section)
	for _, marker := range markers {
		if idx := strings.Index(section, marker); idx != -1 {
			end = idx
			break
		}
	}
	return strings.TrimSpace(section[:end])
}
