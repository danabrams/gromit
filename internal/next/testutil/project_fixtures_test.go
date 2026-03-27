package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
)

func TestWriteMinimalProjectFixtures(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "proj")
	fixtures := WriteMinimalProjectFixtures(t, projectDir)

	wantConfigPath := filepath.Join(projectDir, "project.json")
	if fixtures.ConfigPath != wantConfigPath {
		t.Fatalf("config path = %q, want %q", fixtures.ConfigPath, wantConfigPath)
	}
	wantPolicyPath := filepath.Join(projectDir, "policy", "execution.json")
	if fixtures.PolicyPath != wantPolicyPath {
		t.Fatalf("policy path = %q, want %q", fixtures.PolicyPath, wantPolicyPath)
	}

	data, err := os.ReadFile(fixtures.ConfigPath)
	if err != nil {
		t.Fatalf("read project config: %v", err)
	}
	var cfg ProjectFixtureConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal project config: %v", err)
	}

	if cfg.Name != filepath.Base(projectDir) {
		t.Fatalf("project name = %q, want %q", cfg.Name, filepath.Base(projectDir))
	}
	if cfg.RepoPath != projectDir {
		t.Fatalf("repo path = %q, want %q", cfg.RepoPath, projectDir)
	}
	if cfg.SpecsDir != filepath.Join(projectDir, "docs", "specs") {
		t.Fatalf("specs dir = %q, want %q", cfg.SpecsDir, filepath.Join(projectDir, "docs", "specs"))
	}
	if cfg.DistillerTier != "medium" {
		t.Fatalf("distiller tier = %q, want %q", cfg.DistillerTier, "medium")
	}
	if cfg.CreatedAt.IsZero() {
		t.Fatal("created_at should be non-zero")
	}

	policy, err := execpolicy.LoadPolicy(fixtures.PolicyPath)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if policy.Budgets.MaxSpecCycles != execpolicy.DefaultPolicy().Budgets.MaxSpecCycles {
		t.Fatalf("policy mismatch: got %d, want %d", policy.Budgets.MaxSpecCycles, execpolicy.DefaultPolicy().Budgets.MaxSpecCycles)
	}
}

func TestWriteMinimalProjectFixtures_CustomOptions(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "cells", "custom")
	specsDir := filepath.Join(base, "my-specs")
	policyPath := filepath.Join(base, "policy.json")
	customTime := time.Date(2023, 7, 9, 10, 11, 12, 0, time.UTC)

	customPolicy := execpolicy.DefaultPolicy()
	customPolicy.Models.Planner = "low"

	fixtures := WriteMinimalProjectFixtures(t, projectDir,
		WithProjectName("custom"),
		WithRepoPath("/tmp/repo"),
		WithSpecsDir(specsDir),
		WithDistillerTier("low"),
		WithCreatedAt(customTime),
		WithPolicyPath(policyPath),
		WithPolicy(customPolicy),
	)

	if fixtures.Config.Name != "custom" {
		t.Fatalf("project name = %q, want custom", fixtures.Config.Name)
	}
	if fixtures.Config.RepoPath != "/tmp/repo" {
		t.Fatalf("repo path = %q, want /tmp/repo", fixtures.Config.RepoPath)
	}
	if fixtures.Config.SpecsDir != specsDir {
		t.Fatalf("specs dir = %q, want %q", fixtures.Config.SpecsDir, specsDir)
	}
	if fixtures.Config.DistillerTier != "low" {
		t.Fatalf("distiller tier = %q, want low", fixtures.Config.DistillerTier)
	}
	if !fixtures.Config.CreatedAt.Equal(customTime) {
		t.Fatalf("created_at = %v, want %v", fixtures.Config.CreatedAt, customTime)
	}
	if fixtures.PolicyPath != policyPath {
		t.Fatalf("policy path = %q, want %q", fixtures.PolicyPath, policyPath)
	}
	if fixtures.ProjectDir != projectDir {
		t.Fatalf("project dir = %q, want %q", fixtures.ProjectDir, projectDir)
	}
	if _, err := os.Stat(specsDir); err != nil {
		t.Fatalf("specs dir missing: %v", err)
	}
	if _, err := os.Stat(policyPath); err != nil {
		t.Fatalf("policy file missing: %v", err)
	}

	policy, err := execpolicy.LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if policy.Models.Planner != "low" {
		t.Fatalf("policy planner = %q, want low", policy.Models.Planner)
	}
}
