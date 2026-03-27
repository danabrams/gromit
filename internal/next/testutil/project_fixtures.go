package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
)

// ProjectFixtureOption configures how fixtures are written.
type ProjectFixtureOption func(*projectFixtureOptions)

// ProjectFixtures describes files written by the helper.
type ProjectFixtures struct {
	ProjectDir string
	ConfigPath string
	PolicyPath string
	Config     ProjectFixtureConfig
	Policy     execpolicy.Policy
}

// ProjectFixtureConfig mirrors the minimal project.json shape tests rely on.
type ProjectFixtureConfig struct {
	Name          string    `json:"name"`
	RepoPath      string    `json:"repo_path"`
	SpecsDir      string    `json:"specs_dir,omitempty"`
	DistillerTier string    `json:"distiller_tier,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type projectFixtureOptions struct {
	projectName   string
	repoPath      string
	specsDir      string
	distillerTier string
	createdAt     time.Time
	policy        execpolicy.Policy
	policyPath    string
}

// WriteMinimalProjectFixtures writes a minimal project.json and execution policy fixture into projectDir.
func WriteMinimalProjectFixtures(t testing.TB, projectDir string, opts ...ProjectFixtureOption) ProjectFixtures {
	t.Helper()
	if projectDir == "" {
		t.Fatal("projectDir must be set")
	}

	settings := defaultProjectFixtureOptions(projectDir)
	for _, opt := range opts {
		opt(&settings)
	}

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	if settings.specsDir != "" {
		if err := os.MkdirAll(settings.specsDir, 0o755); err != nil {
			t.Fatalf("mkdir specs dir: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(settings.policyPath), 0o755); err != nil {
		t.Fatalf("mkdir policy dir: %v", err)
	}

	config := ProjectFixtureConfig{
		Name:          settings.projectName,
		RepoPath:      settings.repoPath,
		SpecsDir:      settings.specsDir,
		DistillerTier: settings.distillerTier,
		CreatedAt:     settings.createdAt,
	}

	configPath := filepath.Join(projectDir, "project.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("marshal project config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	policyData, err := json.MarshalIndent(settings.policy, "", "  ")
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	if err := os.WriteFile(settings.policyPath, policyData, 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	return ProjectFixtures{
		ProjectDir: projectDir,
		ConfigPath: configPath,
		PolicyPath: settings.policyPath,
		Config:     config,
		Policy:     settings.policy,
	}
}

func defaultProjectFixtureOptions(projectDir string) projectFixtureOptions {
	return projectFixtureOptions{
		projectName:   defaultProjectName(projectDir),
		repoPath:      projectDir,
		specsDir:      filepath.Join(projectDir, "docs", "specs"),
		distillerTier: "medium",
		createdAt:     time.Now().UTC(),
		policy:        execpolicy.DefaultPolicy(),
		policyPath:    filepath.Join(projectDir, "policy", "execution.json"),
	}
}

func defaultProjectName(projectDir string) string {
	base := filepath.Base(projectDir)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "project"
	}
	return base
}

// WithProjectName overrides the generated project name written to project.json.
func WithProjectName(name string) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		if name != "" {
			o.projectName = name
		}
	}
}

// WithRepoPath overrides repo_path in project.json.
func WithRepoPath(repoPath string) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		if repoPath != "" {
			o.repoPath = repoPath
		}
	}
}

// WithSpecsDir overrides specs_dir in project.json and creates the target directory.
func WithSpecsDir(specsDir string) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		if specsDir != "" {
			o.specsDir = specsDir
		}
	}
}

// WithDistillerTier overrides distiller_tier in project.json.
func WithDistillerTier(tier string) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		if tier != "" {
			o.distillerTier = tier
		}
	}
}

// WithCreatedAt overrides the created_at timestamp in project.json.
func WithCreatedAt(ts time.Time) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		if !ts.IsZero() {
			o.createdAt = ts
		}
	}
}

// WithPolicy overrides the policy written to execution.json.
func WithPolicy(policy execpolicy.Policy) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		o.policy = policy
	}
}

// WithPolicyPath customizes the path where the execution policy is written.
func WithPolicyPath(path string) ProjectFixtureOption {
	return func(o *projectFixtureOptions) {
		if path != "" {
			o.policyPath = path
		}
	}
}
