package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReportInput struct {
	Timestamp string
	Manifest  ManifestMetadata
	Modes     []ModeSummary
}

type ManifestMetadata struct {
	ID         string
	BaseCommit string
	Beads      []string
}

type ModeSummary struct {
	Mode string
}

type ReportPaths struct {
	JSONPath string
	MDPath   string
}

func WriteReport(input ReportInput) (ReportPaths, error) {
	resultDir := filepath.Join(".gromit", "benchmarks", "results", input.Manifest.ID)
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		return ReportPaths{}, fmt.Errorf("create results directory: %w", err)
	}

	jsonPath := filepath.Join(resultDir, input.Timestamp+".json")
	mdPath := filepath.Join(resultDir, input.Timestamp+".md")

	payload := struct {
		ManifestID string        `json:"manifest_id"`
		Modes      []ModeSummary `json:"modes"`
	}{
		ManifestID: input.Manifest.ID,
		Modes:      append([]ModeSummary(nil), input.Modes...),
	}
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ReportPaths{}, fmt.Errorf("marshal report json: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return ReportPaths{}, fmt.Errorf("write report json: %w", err)
	}

	md := strings.Builder{}
	md.WriteString("# Benchmark Report\n")
	md.WriteString("\n")
	md.WriteString("Manifest: " + input.Manifest.ID + "\n")
	if err := os.WriteFile(mdPath, []byte(md.String()), 0o644); err != nil {
		return ReportPaths{}, fmt.Errorf("write report markdown: %w", err)
	}

	return ReportPaths{JSONPath: jsonPath, MDPath: mdPath}, nil
}
