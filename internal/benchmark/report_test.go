package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReport_WritesJSONAndMarkdownArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	_, err := WriteReport(ReportInput{
		Timestamp: "20260223T120000Z",
		Manifest: ManifestMetadata{
			ID:         "tdd-vs-single-pass",
			BaseCommit: "abc123",
			Beads:      []string{"gromit-1", "gromit-2", "gromit-3"},
		},
		Modes: []ModeSummary{{Mode: "single_pass"}},
	})
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.json")
	mdPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.md")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("json artifact missing: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("markdown artifact missing: %v", err)
	}
}
