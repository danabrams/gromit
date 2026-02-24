package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

func TestFilterUndecomposedPlans(t *testing.T) {
	tests := []struct {
		name      string
		planFiles map[string]string // filename -> content (with frontmatter)
		force     bool
		wantNames []string // expected plan names in sorted order
	}{
		{
			name:      "empty directory",
			planFiles: map[string]string{},
			force:     false,
			wantNames: []string{},
		},
		{
			name: "all decomposed",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: true
decomposed_at: "2024-01-15T10:00:00Z"
---
# Plan A
Content here`,
				"plan-b.md": `---
decomposed: true
decomposed_at: "2024-01-16T10:00:00Z"
---
# Plan B
Content here`,
			},
			force:     false,
			wantNames: []string{},
		},
		{
			name: "none decomposed",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: false
---
# Plan A
Content here`,
				"plan-b.md": `---
decomposed: false
---
# Plan B
Content here`,
			},
			force:     false,
			wantNames: []string{"plan-a", "plan-b"},
		},
		{
			name: "mixed decomposed and undecomposed",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: true
decomposed_at: "2024-01-15T10:00:00Z"
---
# Plan A
Content here`,
				"plan-b.md": `---
decomposed: false
---
# Plan B
Content here`,
				"plan-c.md": `---
decomposed: true
decomposed_at: "2024-01-16T10:00:00Z"
---
# Plan C
Content here`,
				"plan-d.md": `---
decomposed: false
---
# Plan D
Content here`,
			},
			force:     false,
			wantNames: []string{"plan-b", "plan-d"},
		},
		{
			name: "force flag includes all plans",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: true
decomposed_at: "2024-01-15T10:00:00Z"
---
# Plan A
Content here`,
				"plan-b.md": `---
decomposed: false
---
# Plan B
Content here`,
				"plan-c.md": `---
decomposed: true
decomposed_at: "2024-01-16T10:00:00Z"
---
# Plan C
Content here`,
			},
			force:     true,
			wantNames: []string{"plan-a", "plan-b", "plan-c"},
		},
		{
			name: "missing frontmatter treated as undecomposed",
			planFiles: map[string]string{
				"plan-a.md": `# Plan A
No frontmatter here`,
				"plan-b.md": `---
decomposed: true
decomposed_at: "2024-01-15T10:00:00Z"
---
# Plan B
Content here`,
			},
			force:     false,
			wantNames: []string{"plan-a"},
		},
		{
			name: "missing decomposed field treated as undecomposed",
			planFiles: map[string]string{
				"plan-a.md": `---
id: some-id
created: "2024-01-15"
---
# Plan A
No decomposed field`,
				"plan-b.md": `---
decomposed: true
decomposed_at: "2024-01-15T10:00:00Z"
---
# Plan B
Content here`,
			},
			force:     false,
			wantNames: []string{"plan-a"},
		},
		{
			name: "non-.md files ignored",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: false
---
# Plan A
Content here`,
				"readme.txt": `This is not a plan`,
				"plan-b.md": `---
decomposed: false
---
# Plan B
Content here`,
				"notes.org": `More non-markdown content`,
			},
			force:     false,
			wantNames: []string{"plan-a", "plan-b"},
		},
		{
			name: "sorting by name",
			planFiles: map[string]string{
				"zebra.md": `---
decomposed: false
---
# Zebra Plan`,
				"alpha.md": `---
decomposed: false
---
# Alpha Plan`,
				"beta.md": `---
decomposed: false
---
# Beta Plan`,
			},
			force:     false,
			wantNames: []string{"alpha", "beta", "zebra"},
		},
		{
			name: "extract title from content",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: false
---
# User Authentication System
This plan covers auth`,
			},
			force:     false,
			wantNames: []string{"plan-a"},
		},
		{
			name: "handles subdirectories by ignoring them",
			planFiles: map[string]string{
				"plan-a.md": `---
decomposed: false
---
# Plan A`,
			},
			force:     false,
			wantNames: []string{"plan-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			plansDir := t.TempDir()

			// Create plan files
			for filename, content := range tt.planFiles {
				planPath := filepath.Join(plansDir, filename)
				if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create plan file %s: %v", filename, err)
				}
			}

			// Create a subdirectory to verify it's ignored
			if tt.name == "handles subdirectories by ignoring them" {
				subdir := filepath.Join(plansDir, "subdir")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					t.Fatalf("failed to create subdirectory: %v", err)
				}
				subdirPlan := filepath.Join(subdir, "plan-in-subdir.md")
				if err := os.WriteFile(subdirPlan, []byte("---\ndecomposed: false\n---\n# Subdir Plan"), 0644); err != nil {
					t.Fatalf("failed to create plan in subdirectory: %v", err)
				}
			}

			// Run the filter
			got, err := filterUndecomposedPlans(plansDir, tt.force)
			if err != nil {
				t.Fatalf("filterUndecomposedPlans() error = %v", err)
			}

			// Extract names from results
			gotNames := make([]string, len(got))
			for i, plan := range got {
				gotNames[i] = plan.Name
			}

			// Compare names
			if len(gotNames) != len(tt.wantNames) {
				t.Errorf("filterUndecomposedPlans() returned %d plans, want %d\ngot:  %v\nwant: %v",
					len(gotNames), len(tt.wantNames), gotNames, tt.wantNames)
				return
			}

			for i, gotName := range gotNames {
				if gotName != tt.wantNames[i] {
					t.Errorf("filterUndecomposedPlans()[%d].Name = %v, want %v", i, gotName, tt.wantNames[i])
				}
			}

			// Verify title extraction for the specific test case
			if tt.name == "extract title from content" && len(got) > 0 {
				if got[0].Title != "User Authentication System" {
					t.Errorf("filterUndecomposedPlans()[0].Title = %v, want %v", got[0].Title, "User Authentication System")
				}
			}

			// Verify paths are set correctly
			for i, plan := range got {
				expectedPath := filepath.Join(plansDir, plan.Name+".md")
				if plan.Path != expectedPath {
					t.Errorf("filterUndecomposedPlans()[%d].Path = %v, want %v", i, plan.Path, expectedPath)
				}
			}
		})
	}
}

func TestFilterUndecomposedPlans_NonexistentDirectory(t *testing.T) {
	// Use a path that doesn't exist
	nonexistentDir := filepath.Join(t.TempDir(), "does-not-exist")

	got, err := filterUndecomposedPlans(nonexistentDir, false)
	if err != nil {
		t.Errorf("filterUndecomposedPlans() with nonexistent dir should not error, got error = %v", err)
	}

	if len(got) != 0 {
		t.Errorf("filterUndecomposedPlans() with nonexistent dir should return empty slice, got %d plans", len(got))
	}
}

func TestFilterUndecomposedPlans_UnreadableFile(t *testing.T) {
	plansDir := t.TempDir()

	// Create a valid plan
	validPlan := filepath.Join(plansDir, "valid.md")
	if err := os.WriteFile(validPlan, []byte("---\ndecomposed: false\n---\n# Valid"), 0644); err != nil {
		t.Fatalf("failed to create valid plan: %v", err)
	}

	// Create a file with invalid frontmatter (should be skipped)
	invalidPlan := filepath.Join(plansDir, "invalid.md")
	if err := os.WriteFile(invalidPlan, []byte("---\ninvalid: yaml: content:\n---\n# Invalid"), 0644); err != nil {
		t.Fatalf("failed to create invalid plan: %v", err)
	}

	// Run the filter
	got, err := filterUndecomposedPlans(plansDir, false)
	if err != nil {
		t.Fatalf("filterUndecomposedPlans() error = %v", err)
	}

	// Should only return the valid plan, skipping the invalid one
	if len(got) != 1 {
		t.Errorf("filterUndecomposedPlans() returned %d plans, want 1 (invalid should be skipped)", len(got))
	}

	if len(got) > 0 && got[0].Name != "valid" {
		t.Errorf("filterUndecomposedPlans()[0].Name = %v, want 'valid'", got[0].Name)
	}
}

func TestDecomposeSinglePlan_ReviewUsesSessionWorktreeDir(t *testing.T) {
	origReview := decomposeReview
	origLauncher := decomposeSessionLauncherFn
	origRunInDir := decomposeRunInDirFn
	origCurrentDirFn := decomposeSinglePlanInDirFn
	t.Cleanup(func() {
		decomposeReview = origReview
		decomposeSessionLauncherFn = origLauncher
		decomposeRunInDirFn = origRunInDir
		decomposeSinglePlanInDirFn = origCurrentDirFn
	})

	mainDir := t.TempDir()
	sessionDir := filepath.Join(mainDir, "session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("creating session dir: %v", err)
	}

	decomposeReview = true
	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(mainDir, ".gromit")

	gotCommand := ""
	calledInSessionDir := ""
	decomposeSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		gotCommand = command
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/decompose-test", WorktreeDir: sessionDir}, nil
	}
	decomposeSinglePlanInDirFn = func(planName string, cfg *config.Config) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		calledInSessionDir = cwd
		return nil
	}

	if err := decomposeSinglePlan("plan-a", cfg); err != nil {
		t.Fatalf("decomposeSinglePlan() error = %v", err)
	}
	if gotCommand != "decompose" {
		t.Fatalf("command = %q, want %q", gotCommand, "decompose")
	}
	if calledInSessionDir != sessionDir {
		t.Fatalf("decompose executed in %q, want %q", calledInSessionDir, sessionDir)
	}
}

func TestDecomposeSinglePlan_ReviewConflictHandoffPropagates(t *testing.T) {
	origReview := decomposeReview
	origLauncher := decomposeSessionLauncherFn
	origCurrentDirFn := decomposeSinglePlanInDirFn
	t.Cleanup(func() {
		decomposeReview = origReview
		decomposeSessionLauncherFn = origLauncher
		decomposeSinglePlanInDirFn = origCurrentDirFn
	})

	decomposeReview = true
	cfg := &config.Config{}
	cfg.Paths.GromitDir = filepath.Join(t.TempDir(), ".gromit")

	currentDirCalled := false
	decomposeSinglePlanInDirFn = func(planName string, cfg *config.Config) error {
		currentDirCalled = true
		return nil
	}

	decomposeSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		return &worktree.SessionWorktree{
				BranchName:  "gromit/decompose-conflict",
				WorktreeDir: "/tmp/session-decompose",
			}, &mergeConflictHandoffError{
				Policy:     conflictPolicyManual,
				Branch:     "gromit/decompose-conflict",
				SessionDir: "/tmp/session-decompose",
				MergeErr:   errors.New("merge conflict"),
			}
	}

	err := decomposeSinglePlan("plan-b", cfg)
	if err == nil {
		t.Fatal("expected conflict handoff error, got nil")
	}
	if !isMergeConflictHandoffError(err) {
		t.Fatalf("expected merge conflict handoff error, got %T (%v)", err, err)
	}
	if currentDirCalled {
		t.Fatal("decompose execution should not continue when launcher returns conflict handoff")
	}
}

func TestBuildDecomposeClient_CodexProviderPath(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5-mini",
				},
			},
		},
		Claude: config.ClaudeConfig{
			PipelineTimeout: 789,
		},
	}

	client, err := buildDecomposeClient(cfg)
	if err != nil {
		t.Fatalf("buildDecomposeClient() error = %v", err)
	}

	typedClient, ok := client.(*llmRouterClientAdapter)
	if !ok {
		t.Fatalf("client type = %T, want *llmRouterClientAdapter", client)
	}
	if typedClient.Timeout != 789*time.Second {
		t.Fatalf("timeout = %v, want %v", typedClient.Timeout, 789*time.Second)
	}
	if typedClient.Phase != decomposeSessionCommand {
		t.Fatalf("phase = %q, want %q", typedClient.Phase, decomposeSessionCommand)
	}
	if typedClient.Router == nil {
		t.Fatal("router = nil, want non-nil")
	}
}

func TestBuildDecomposeClient_ClaudeFallbackPath(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 123,
		},
	}

	client, err := buildDecomposeClient(cfg)
	if err != nil {
		t.Fatalf("buildDecomposeClient() error = %v", err)
	}

	typedClient, ok := client.(*claudeClientAdapter)
	if !ok {
		t.Fatalf("client type = %T, want *claudeClientAdapter", client)
	}
	if typedClient.Timeout != time.Duration(config.DefaultPipelineTimeoutSeconds)*time.Second {
		t.Fatalf("timeout = %v, want %v", typedClient.Timeout, time.Duration(config.DefaultPipelineTimeoutSeconds)*time.Second)
	}
}

func TestBuildDecomposeClient_ProviderRouterPath_DefaultPipelineTimeout(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5-mini",
				},
			},
		},
		Claude: config.ClaudeConfig{},
	}

	client, err := buildDecomposeClient(cfg)
	if err != nil {
		t.Fatalf("buildDecomposeClient() error = %v", err)
	}

	typedClient, ok := client.(*llmRouterClientAdapter)
	if !ok {
		t.Fatalf("client type = %T, want *llmRouterClientAdapter", client)
	}
	if typedClient.Timeout != time.Duration(config.DefaultPipelineTimeoutSeconds)*time.Second {
		t.Fatalf("timeout = %v, want %v", typedClient.Timeout, time.Duration(config.DefaultPipelineTimeoutSeconds)*time.Second)
	}
	if typedClient.Phase != decomposeSessionCommand {
		t.Fatalf("phase = %q, want %q", typedClient.Phase, decomposeSessionCommand)
	}
}

func TestBuildDecomposeInput_UsesConfiguredTier(t *testing.T) {
	origForce := decomposeForce
	origReview := decomposeReview
	origSkipValidation := decomposeSkipValidation
	origMaxRetries := decomposeMaxRetries
	t.Cleanup(func() {
		decomposeForce = origForce
		decomposeReview = origReview
		decomposeSkipValidation = origSkipValidation
		decomposeMaxRetries = origMaxRetries
	})

	decomposeForce = true
	decomposeReview = false
	decomposeSkipValidation = true
	decomposeMaxRetries = -1

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			MaxValidationRetries: 7,
		},
		Decompose: config.DecomposeConfig{
			Tier: "high",
		},
	}

	input := buildDecomposeInput("tier-plan", cfg)
	if input.Tier != "high" {
		t.Fatalf("Tier = %q, want %q", input.Tier, "high")
	}
}

func TestReconcilePlanDecomposedState_MarksPlanWhenSpecBeadsExist(t *testing.T) {
	origListWithLabel := decomposeListWithLabelFn
	t.Cleanup(func() {
		decomposeListWithLabelFn = origListWithLabel
	})

	plansDir := t.TempDir()
	planPath := filepath.Join(plansDir, "sample.md")
	content := `---
id: sample
decomposed: false
---
# Sample plan`
	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing plan file: %v", err)
	}

	decomposeListWithLabelFn = func(label string) ([]*bead.Bead, error) {
		if label != "spec:sample" {
			t.Fatalf("label = %q, want %q", label, "spec:sample")
		}
		return []*bead.Bead{{ID: "gromit-1", Title: "Task"}}, nil
	}

	alreadyDecomposed, reconciled, err := reconcilePlanDecomposedState(planPath, "sample", false)
	if err != nil {
		t.Fatalf("reconcilePlanDecomposedState() error = %v", err)
	}
	if !alreadyDecomposed {
		t.Fatal("expected plan to be considered decomposed after reconciliation")
	}
	if !reconciled {
		t.Fatal("expected reconciliation flag to be true")
	}

	gotBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading updated plan file: %v", err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "decomposed: true") {
		t.Fatalf("updated plan missing decomposed: true\n%s", got)
	}
	if !strings.Contains(got, "decomposed_at:") {
		t.Fatalf("updated plan missing decomposed_at\n%s", got)
	}
}

func TestFilterUndecomposedPlans_ReconcilesAndSkipsDecomposedPlans(t *testing.T) {
	origListWithLabel := decomposeListWithLabelFn
	t.Cleanup(func() {
		decomposeListWithLabelFn = origListWithLabel
	})

	plansDir := t.TempDir()
	planPath := filepath.Join(plansDir, "sample.md")
	content := `---
id: sample
decomposed: false
---
# Sample plan`
	if err := os.WriteFile(planPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing plan file: %v", err)
	}

	decomposeListWithLabelFn = func(label string) ([]*bead.Bead, error) {
		if label != "spec:sample" {
			return []*bead.Bead{}, nil
		}
		return []*bead.Bead{{ID: "gromit-1", Title: "Task"}}, nil
	}

	plans, err := filterUndecomposedPlans(plansDir, false)
	if err != nil {
		t.Fatalf("filterUndecomposedPlans() error = %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("expected reconciled plan to be skipped, got %d plan(s)", len(plans))
	}

	gotBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading updated plan file: %v", err)
	}
	if !strings.Contains(string(gotBytes), "decomposed: true") {
		t.Fatalf("expected plan to be marked decomposed, got:\n%s", string(gotBytes))
	}
}
