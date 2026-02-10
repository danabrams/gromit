package main

import (
	"os"
	"path/filepath"
	"strings"
)

// getEpicFiles returns a list of .md files in the epics directory
func getEpicFiles(epicsDir string) ([]string, error) {
	entries, err := os.ReadDir(epicsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	epics := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			epics = append(epics, filepath.Join(epicsDir, entry.Name()))
		}
	}

	return epics, nil
}
