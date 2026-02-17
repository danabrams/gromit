package runner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RunnerSmokeMatrixEntry represents a documented smoke-matrix decision.
type RunnerSmokeMatrixEntry struct {
	Decision    string
	Rationale   string
	Destination string
}

// LoadRunnerSmokeMatrix loads the consolidated smoke matrix documentation and
// returns entries keyed by "file:TestName".
func LoadRunnerSmokeMatrix(projectRoot string) (map[string]RunnerSmokeMatrixEntry, error) {
	path := filepath.Join(projectRoot, "docs", "smoke_coverage_matrix.md")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open smoke coverage matrix: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	entries := make(map[string]RunnerSmokeMatrixEntry)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}
		row := strings.Trim(line, "|")
		parts := strings.Split(row, "|")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid smoke matrix row %q: expected 4 columns, got %d", line, len(parts))
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if parts[0] == "" || parts[0] == "case" || strings.HasPrefix(parts[0], "---") {
			continue
		}

		caseID := parts[0]
		if !strings.HasPrefix(caseID, "internal/runner/") {
			continue
		}
		if _, ok := entries[caseID]; ok {
			return nil, fmt.Errorf("duplicate smoke matrix case %q", caseID)
		}
		entries[caseID] = RunnerSmokeMatrixEntry{
			Decision:    parts[1],
			Rationale:   parts[2],
			Destination: parts[3],
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan smoke coverage matrix: %w", err)
	}

	return entries, nil
}
