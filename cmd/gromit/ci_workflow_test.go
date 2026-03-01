package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkflow_HasPRMetadataValidationJob(t *testing.T) {
	// Read CI workflow file from repo root
	repoRoot := findRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "ci.yml")
	content, err := ioutil.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", workflowPath, err)
	}

	workflowText := string(content)

	// Check that the workflow includes validate PR metadata job
	if !strings.Contains(workflowText, "validate") && !strings.Contains(workflowText, "validate-pr-metadata") {
		t.Fatalf("CI workflow should include a step or job for PR metadata validation")
	}

	// Check that it runs on pull_request event
	if !strings.Contains(workflowText, "pull_request") {
		t.Fatalf("CI workflow should run on pull_request event")
	}

	// Check that it calls the validate-pr-metadata command
	if !strings.Contains(workflowText, "validate-pr-metadata") {
		t.Fatalf("CI workflow should call 'validate-pr-metadata' command")
	}
}

func TestCIWorkflow_PassesPRBodyEnvironment(t *testing.T) {
	// Read CI workflow file from repo root
	repoRoot := findRepoRoot(t)
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "ci.yml")
	content, err := ioutil.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", workflowPath, err)
	}

	workflowText := string(content)

	// Check that PR_BODY environment variable is set from github context
	if !strings.Contains(workflowText, "PR_BODY") && !strings.Contains(workflowText, "github.event.pull_request.body") {
		t.Fatalf("CI workflow should pass PR body via PR_BODY environment variable or github.event.pull_request.body")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	// Start from current directory and walk up to find go.mod
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatalf("could not find repo root (go.mod)")
		}
		cwd = parent
	}
}
