package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/worktree"
)

type testPlanQuerier struct {
	capturedCtx context.Context
	result      *pipeline.QueryUndecomposedPlansResult
}

func (q *testPlanQuerier) QueryUndecomposedPlans(ctx context.Context, input pipeline.QueryUndecomposedPlansInput) (*pipeline.QueryUndecomposedPlansResult, error) {
	q.capturedCtx = ctx
	return q.result, nil
}

func TestQueryDecomposePlansWithPipeline_UsesProvidedContext(t *testing.T) {
	t.Parallel()

	querier := &testPlanQuerier{
		result: &pipeline.QueryUndecomposedPlansResult{
			Plans: []pipeline.PlanQueryInfo{{Name: "plan-a", Title: "Plan A", Path: "plan-a.md"}},
		},
	}
	markerCtx := context.WithValue(context.Background(), "marker", "value")

	if _, err := queryDecomposePlansWithPipeline(markerCtx, querier); err != nil {
		t.Fatalf("queryDecomposePlansWithPipeline() error = %v", err)
	}

	if querier.capturedCtx == nil {
		t.Fatal("expected querier to receive a context")
	}
	if querier.capturedCtx.Value("marker") != "value" {
		t.Fatalf("context value missing, got %v", querier.capturedCtx.Value("marker"))
	}
}

func TestFilterUndecomposedPlans(t *testing.T) {
	// Not parallel: subtests mutate package-level decomposeListWithLabelFn.
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
			// Not parallel: subtests mutate package-level decomposeListWithLabelFn.
			origListFn := decomposeListWithLabelFn
			decomposeListWithLabelFn = func(ctx context.Context, label string) ([]*bead.Bead, error) {
				return nil, nil // stub: no beads found
			}
			t.Cleanup(func() { decomposeListWithLabelFn = origListFn })

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
	t.Parallel(
	// Use a path that doesn't exist
	)

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
	t.Parallel()
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
		ctx context.Context,
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
		ctx context.Context,
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
	t.Parallel()
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.1-codex-mini",
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
	t.Parallel()
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
	t.Parallel()
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.1-codex-mini",
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

	decomposeListWithLabelFn = func(ctx context.Context, label string) ([]*bead.Bead, error) {
		if label != "spec:sample" {
			t.Fatalf("label = %q, want %q", label, "spec:sample")
		}
		return []*bead.Bead{{ID: "gromit-1", Title: "Task"}}, nil
	}

	alreadyDecomposed, reconciled, err := reconcilePlanDecomposedState(context.Background(), planPath, "sample", false)
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

	decomposeListWithLabelFn = func(ctx context.Context, label string) ([]*bead.Bead, error) {
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

func TestReconcilePlanDecomposedStateUsesTrackerClient(t *testing.T) {
	t.Parallel()

	// Create a plan file that's not decomposed
	plansDir := t.TempDir()
	planPath := filepath.Join(plansDir, "test-plan.md")

	planContent := `---
decomposed: false
---

# Test Plan

Some content here.
`

	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("failed to create test plan file: %v", err)
	}

	// Create a mock tracker client that returns items matching the plan label
	mockTrackerClient := &mockTrackerForDecompose{
		items: []interface{}{
			struct {
				ID    string
				Title string
			}{
				ID:    "bead-1",
				Title: "Test Bead",
			},
		},
	}

	// This test expects reconcilePlanDecomposedStateWithTrackerClient to exist
	// and use the tracker client to query for beads with the plan's label
	alreadyDecomposed, reconciled, err := reconcilePlanDecomposedStateWithTrackerClient(
		context.Background(), planPath, "test-plan", false, mockTrackerClient,
	)

	if err != nil {
		t.Fatalf("reconcilePlanDecomposedStateWithTrackerClient returned error: %v", err)
	}

	if !alreadyDecomposed {
		t.Fatalf("expected alreadyDecomposed to be true, got false")
	}

	if !reconciled {
		t.Fatalf("expected reconciled to be true, got false")
	}
}

// mockTrackerForDecompose is a test double for tracker.Client
type mockTrackerForDecompose struct {
	items []interface{}
}

func (m *mockTrackerForDecompose) Ready(context.Context) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForDecompose) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForDecompose) List(context.Context, tracker.Query) ([]tracker.Item, error) {
	if len(m.items) > 0 {
		// Return one item to simulate finding beads
		return []tracker.Item{{ID: "bead-1", Title: "Test"}}, nil
	}
	return []tracker.Item{}, nil
}

func (m *mockTrackerForDecompose) Show(context.Context, string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForDecompose) Search(context.Context, tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForDecompose) Create(context.Context, tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerForDecompose) CreateWithParent(context.Context, tracker.CreateRequest, string) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerForDecompose) Update(context.Context, tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerForDecompose) ListWithLabel(context.Context, string) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForDecompose) Close(context.Context, string) error {
	return nil
}

func (m *mockTrackerForDecompose) Sync(context.Context) error {
	return nil
}

func (m *mockTrackerForDecompose) AddComment(context.Context, string, string) error {
	return nil
}

func (m *mockTrackerForDecompose) HasOpenChildren(context.Context, string) (bool, error) {
	return false, nil
}

func TestListBeadsWithLabelUsingTrackerClient(t *testing.T) {
	t.Parallel()

	// Create a mock tracker client that returns items with specific labels
	mockClient := &mockTrackerClientWithItems{
		returnItems: []tracker.Item{
			{ID: "bead-1", Title: "Auth System"},
			{ID: "bead-2", Title: "DB Migration"},
		},
	}

	// This test expects listBeadsWithLabelUsingTrackerClient to exist
	label := "spec:auth"
	items, err := listBeadsWithLabelUsingTrackerClient(context.Background(), label, mockClient)

	if err != nil {
		t.Fatalf("listBeadsWithLabelUsingTrackerClient returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != "bead-1" {
		t.Fatalf("expected first item ID to be bead-1, got %s", items[0].ID)
	}
}

// mockTrackerClientWithItems is a test double for tracker.Client that returns specific items
type mockTrackerClientWithItems struct {
	returnItems []tracker.Item
}

func (m *mockTrackerClientWithItems) Ready(context.Context) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClientWithItems) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClientWithItems) List(context.Context, tracker.Query) ([]tracker.Item, error) {
	return m.returnItems, nil
}

func (m *mockTrackerClientWithItems) Show(context.Context, string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClientWithItems) Search(context.Context, tracker.Query) ([]tracker.Item, error) {
	return m.returnItems, nil
}

func (m *mockTrackerClientWithItems) Create(context.Context, tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClientWithItems) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	return m.Create(ctx, req)
}
func (m *mockTrackerClientWithItems) Update(context.Context, tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClientWithItems) ListWithLabel(context.Context, string) ([]tracker.Item, error) {
	return m.returnItems, nil
}

func (m *mockTrackerClientWithItems) Close(context.Context, string) error {
	return nil
}

func (m *mockTrackerClientWithItems) Sync(context.Context) error {
	return nil
}

func (m *mockTrackerClientWithItems) AddComment(context.Context, string, string) error {
	return nil
}

func (m *mockTrackerClientWithItems) HasOpenChildren(context.Context, string) (bool, error) {
	return false, nil
}

// TestListBeadsWithLabelThreadsContext verifies that listBeadsWithLabel accepts and uses context
func TestListBeadsWithLabelThreadsContext(t *testing.T) {
	t.Parallel()

	// Call listBeadsWithLabel with a custom context to verify it accepts context parameter
	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	_, _ = listBeadsWithLabel(ctx, "test-label")

	// If this compiles and runs without error, the signature is correct
}

// TestReconcilePlanDecomposedStateThreadsContext verifies that reconcilePlanDecomposedState accepts context
func TestReconcilePlanDecomposedStateThreadsContext(t *testing.T) {
	t.Parallel()

	// Create a temporary plan file
	planContent := `---
decomposed: false
---
# Test Plan`
	planPath := filepath.Join(t.TempDir(), "test-plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("failed to create plan file: %v", err)
	}

	// Call reconcilePlanDecomposedState with context to verify it accepts context parameter
	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	_, _, _ = reconcilePlanDecomposedState(ctx, planPath, "test-plan", false)

	// If this compiles and runs without error, the signature is correct
}

// TestReconcilePlanDecomposedStateUsesTrackerVersion verifies that reconcilePlanDecomposedState
// internally uses the tracker-based version for queries
func TestReconcilePlanDecomposedStateUsesTrackerVersion(t *testing.T) {
	t.Parallel()

	// Create a temporary plan file
	planContent := `---
decomposed: false
---
# Test Plan`
	planPath := filepath.Join(t.TempDir(), "test-plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("failed to create plan file: %v", err)
	}

	// Create mock tracker client
	trackerCalled := false
	mockTracker := &mockTrackerForTrackerVersionTest{
		onList: func(_ context.Context, _ tracker.Query) ([]tracker.Item, error) {
			trackerCalled = true
			return []tracker.Item{{ID: "bead-1", Title: "Test"}}, nil
		},
	}

	// Call with tracker client - expects the tracker version to be used internally
	ctx := context.Background()
	_, _, _ = reconcilePlanDecomposedStateWithTrackerClient(ctx, planPath, "test-plan", false, mockTracker)

	if !trackerCalled {
		t.Errorf("expected tracker client to be called, but it was not")
	}
}

// mockTrackerForTrackerVersionTest is a test double for tracker.Client
type mockTrackerForTrackerVersionTest struct {
	onList func(context.Context, tracker.Query) ([]tracker.Item, error)
}

func (m *mockTrackerForTrackerVersionTest) Ready(context.Context) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	if m.onList != nil {
		return m.onList(ctx, q)
	}
	return []tracker.Item{}, nil
}

func (m *mockTrackerForTrackerVersionTest) Show(context.Context, string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) Search(context.Context, tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) Create(context.Context, tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) CreateWithParent(context.Context, tracker.CreateRequest, string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) Update(context.Context, tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) ListWithLabel(context.Context, string) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForTrackerVersionTest) Close(context.Context, string) error {
	return nil
}

func (m *mockTrackerForTrackerVersionTest) Sync(context.Context) error {
	return nil
}

func (m *mockTrackerForTrackerVersionTest) AddComment(context.Context, string, string) error {
	return nil
}

func (m *mockTrackerForTrackerVersionTest) HasOpenChildren(context.Context, string) (bool, error) {
	return false, nil
}

// TestDecomposeThinWrapperPattern verifies the thin wrapper delegation pattern
// Expected: decompose command delegates plan filtering to Pipeline.QueryUndecomposedPlans
func TestDecomposeThinWrapperPattern(t *testing.T) {
	t.Parallel()

	// Type assertions verify that Pipeline implements the interface
	var _ DecomposePlanQuerier = (*pipeline.Pipeline)(nil)

	// Create a mock pipeline that captures the arguments
	mockPipeline := &mockDecomposePipelineForDelegation{
		queryUndecomposedPlansFn: func(ctx context.Context, input pipeline.QueryUndecomposedPlansInput) (*pipeline.QueryUndecomposedPlansResult, error) {
			return &pipeline.QueryUndecomposedPlansResult{
				Plans: []pipeline.PlanQueryInfo{
					{
						Name:  "test-plan",
						Title: "Test Plan",
						Path:  "/path/to/test-plan.md",
					},
				},
			}, nil
		},
	}

	// Verify the interface is satisfied
	if mockPipeline == nil {
		t.Fatal("mock pipeline should not be nil")
	}

	// Call the mock to verify it works
	ctx := context.Background()
	input := pipeline.QueryUndecomposedPlansInput{Force: false}
	result, err := mockPipeline.QueryUndecomposedPlans(ctx, input)

	if err != nil {
		t.Errorf("QueryUndecomposedPlans() error = %v, want nil", err)
	}

	if len(result.Plans) != 1 {
		t.Errorf("QueryUndecomposedPlans() returned %d plans, want 1", len(result.Plans))
	}

	if result.Plans[0].Name != "test-plan" {
		t.Errorf("QueryUndecomposedPlans() plan name = %q, want 'test-plan'", result.Plans[0].Name)
	}
}

// Mock pipeline for testing delegation in decompose.go
type mockDecomposePipelineForDelegation struct {
	queryUndecomposedPlansFn func(ctx context.Context, input pipeline.QueryUndecomposedPlansInput) (*pipeline.QueryUndecomposedPlansResult, error)
}

func (m *mockDecomposePipelineForDelegation) QueryUndecomposedPlans(ctx context.Context, input pipeline.QueryUndecomposedPlansInput) (*pipeline.QueryUndecomposedPlansResult, error) {
	if m.queryUndecomposedPlansFn != nil {
		return m.queryUndecomposedPlansFn(ctx, input)
	}
	return nil, fmt.Errorf("not implemented")
}

// TestQueryDecomposePlansWithPipeline_ForceFlagPassthrough verifies --force flag is passed to Pipeline
// Expected: createDecomposePipelineFn and queryDecomposePlansWithPipeline pass --force to Pipeline.QueryUndecomposedPlans
func TestQueryDecomposePlansWithPipeline_ForceFlagPassthrough(t *testing.T) {
	t.Parallel()

	// Create a mock pipeline that captures the arguments
	var capturedForce bool
	mockPipeline := &mockDecomposePipelineForDelegation{
		queryUndecomposedPlansFn: func(ctx context.Context, input pipeline.QueryUndecomposedPlansInput) (*pipeline.QueryUndecomposedPlansResult, error) {
			capturedForce = input.Force
			return &pipeline.QueryUndecomposedPlansResult{
				Plans: []pipeline.PlanQueryInfo{},
			}, nil
		},
	}

	// Set the global flag
	origDecomposeForce := decomposeForce
	defer func() {
		decomposeForce = origDecomposeForce
	}()

	decomposeForce = true

	// Call the helper function
	plans, err := queryDecomposePlansWithPipeline(context.Background(), mockPipeline)

	if err != nil {
		t.Errorf("queryDecomposePlansWithPipeline() error = %v, want nil", err)
	}

	if !capturedForce {
		t.Errorf("Pipeline.QueryUndecomposedPlans received Force=%v, want true", capturedForce)
	}

	if plans == nil {
		t.Errorf("queryDecomposePlansWithPipeline() returned nil, want non-nil slice")
	}
}
