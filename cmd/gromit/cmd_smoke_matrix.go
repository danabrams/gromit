package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CmdSmokeMatrixEntry describes a decision for a command acceptance case.
type CmdSmokeMatrixEntry struct {
	Decision    string
	Rationale   string
	Destination string
}

// LoadCmdSmokeMatrix loads cmd/gromit entries from the consolidated smoke coverage matrix.
func LoadCmdSmokeMatrix(projectRoot string) (map[string]CmdSmokeMatrixEntry, error) {
	matrixPath := filepath.Join(projectRoot, "docs", "smoke_coverage_matrix.md")
	f, err := os.Open(matrixPath)
	if err != nil {
		return nil, fmt.Errorf("open smoke coverage matrix: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	entries := make(map[string]CmdSmokeMatrixEntry)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}

		fields := splitSmokeMatrixLine(line)
		if len(fields) != 4 {
			continue
		}
		caseID := fields[0]
		if caseID == "case" || caseID == "Case" {
			continue
		}
		if !strings.HasPrefix(caseID, "cmd/gromit/") {
			continue
		}

		if _, exists := entries[caseID]; exists {
			return nil, fmt.Errorf("duplicate smoke matrix case %s", caseID)
		}

		entries[caseID] = CmdSmokeMatrixEntry{
			Decision:    fields[1],
			Rationale:   fields[2],
			Destination: fields[3],
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan smoke coverage matrix: %w", err)
	}

	return entries, nil
}

func splitSmokeMatrixLine(line string) []string {
	parts := strings.Split(line, "|")
	fields := make([]string, 0, 4)
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		fields = append(fields, trimmed)
	}
	return fields
}
