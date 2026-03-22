package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// loadConfigTier reads the distiller_tier field from project.json.
func loadConfigTier(projectPath string) (reviewdistiller.Tier, error) {
	projectData, err := os.ReadFile(projectPath)
	if err != nil {
		return "", err
	}
	var projectConfig map[string]interface{}
	if err := json.Unmarshal(projectData, &projectConfig); err != nil {
		return "", err
	}
	tierStr, ok := projectConfig["distiller_tier"].(string)
	if !ok {
		return "", fmt.Errorf("distiller_tier not found or not a string in project.json")
	}
	return reviewdistiller.Tier(tierStr), nil
}
