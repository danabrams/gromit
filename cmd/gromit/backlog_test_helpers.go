package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backlogIdea is a minimal struct for reading backlog entries in tests.
type backlogIdea struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Type    string `json:"type"`
	Context string `json:"context"`
}

// loadBacklogIdeas reads and parses all ideas from the project's backlog.jsonl file.
func loadBacklogIdeas(t *testing.T) []backlogIdea {
	t.Helper()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	backlogPath := filepath.Join(projectRoot, ".gromit", "backlog.jsonl")
	data, err := os.ReadFile(backlogPath)
	if err != nil {
		t.Fatalf("could not read backlog.jsonl: %v", err)
	}

	var ideas []backlogIdea
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var idea backlogIdea
		if err := json.Unmarshal([]byte(line), &idea); err != nil {
			t.Fatalf("failed to parse backlog line: %v", err)
		}
		ideas = append(ideas, idea)
	}

	return ideas
}

// containsAny returns true if text contains any of the given substrings (case-insensitive).
func containsAny(text string, substrs ...string) bool {
	for _, s := range substrs {
		if strings.Contains(text, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// findProjectRoot walks up from the current directory to find the project root
// (identified by the presence of gromit.yaml or .gromit/ directory).
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gromit.yaml")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".gromit")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
