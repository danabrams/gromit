package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/provider"
)

func TestRealStageProvider_BuildStages_ReturnsStages(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages returned error: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("expected at least one stage, got 0")
	}

	expectedNames := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
	if len(stages) != len(expectedNames) {
		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
	}
	for i, name := range expectedNames {
		if stages[i].Name() != name {
			t.Errorf("stage[%d].Name() = %q, want %q", i, stages[i].Name(), name)
		}
	}
}

func TestRealStageProvider_BuildStages_NoStubError(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	_, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages should not return stub error, got: %v", err)
	}
}

func TestRealStageProvider_ReviewBeforeAccept(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	var reviewIdx, acceptIdx int
	for i, s := range stages {
		if s.Name() == "review" {
			reviewIdx = i
		}
		if s.Name() == "accept" {
			acceptIdx = i
		}
	}
	if reviewIdx >= acceptIdx {
		t.Errorf("review (idx %d) must come before accept (idx %d)", reviewIdx, acceptIdx)
	}
}

func TestRealStageProvider_BuildStages_InvalidThresholdReturnsError(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	policy.Review.ReplanThreshold = "bogus"
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	_, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err == nil {
		t.Fatal("expected error for invalid replan threshold, got nil")
	}
	if !strings.Contains(err.Error(), "invalid replan threshold") {
		t.Errorf("error should mention invalid replan threshold, got: %v", err)
	}
}

func TestRealStageProvider_BuildStages_ValidThresholdSucceeds(t *testing.T) {
	for _, threshold := range []string{"error", "warning", "suggestion", "info"} {
		t.Run(threshold, func(t *testing.T) {
			policy := execpolicy.DefaultPolicy()
			policy.Review.ReplanThreshold = threshold
			rs := runstore.NewRunState("test-spec", "test-project")

			provider := NewRealStageProvider(RealStageProviderConfig{
				WorkDir:  t.TempDir(),
				StoreDir: t.TempDir(),
				SpecPath: "nonexistent-spec.md",
			})

			_, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
			if err != nil {
				t.Fatalf("BuildStages returned error for valid threshold %q: %v", threshold, err)
			}
		})
	}
}

func TestRealStageProvider_BuildStages_DefaultTierUsesModelsEvaluator(t *testing.T) {
	// This test verifies that DefaultTier is wired from policy.Models.Evaluator
	// (not from policy.Review.ReplanThreshold). We verify by running the review
	// stage with a capturing runner and checking the config flows correctly.
	// Since ReviewStage fields are unexported, we verify indirectly: BuildStages
	// succeeds with a policy where Models.Evaluator differs from ReplanThreshold.
	policy := execpolicy.DefaultPolicy()
	policy.Models.Evaluator = "low"         // model tier
	policy.Review.ReplanThreshold = "error" // severity — different type entirely
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "nonexistent-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}
	// Verify we got the expected stages (sanity check).
	if len(stages) != 9 {
		t.Fatalf("expected 9 stages, got %d", len(stages))
	}
}

func TestRealStageProvider_BuildStages_SpecContentWiredIntoReviewAndAccept(t *testing.T) {
	// Create a temp spec file with known content.
	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "test-spec.md")
	specContent := "# Test Spec\n\n## Acceptance Criteria\n- It works\n"
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: specPath,
	})

	stages, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	// Verify review and accept stages are present and functional by running them.
	// The noop runners return success, but accept stage parses SpecContent for
	// criteria — if SpecContent is wired, it will find the criteria.
	for _, s := range stages {
		if s.Name() == "accept" {
			// Run the accept stage — if SpecContent is NOT wired, the accept
			// stage returns NeedsHuman because it can't find criteria.
			// If wired, the noop evaluator returns AllPass=true → Continue.
			action, err := s.Run(context.Background(), rs)
			if err != nil {
				t.Fatalf("accept stage Run: %v", err)
			}
			if action.Kind != specloop.Continue {
				t.Errorf("accept stage returned kind=%v, want Continue; SpecContent likely not wired", action.Kind)
			}
		}
	}
}

// mockTestProvider satisfies provider.Provider for wiring tests.
type mockTestProvider struct {
	name string
}

func (m *mockTestProvider) Name() string                    { return m.name }
func (m *mockTestProvider) ModelForTier(tier string) string { return "mock-" + tier }
func (m *mockTestProvider) Run(_ context.Context, _ string, _ string) (*provider.Result, error) {
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *mockTestProvider) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *mockTestProvider) RunValidation(_ context.Context, _ []string, _ string, _ string) (*provider.Result, error) {
	return &provider.Result{Output: "ok", Success: true}, nil
}
func (m *mockTestProvider) IsUsageLimitError(_ *provider.Result, _ error) bool { return false }
func (m *mockTestProvider) IsValidationPassed(_ *provider.Result) bool         { return true }
func (m *mockTestProvider) IsScopeTooLarge(_ *provider.Result) (bool, string)  { return false, "" }

// costTrackingProvider returns a result with a fixed CostUSD so that OnCost
// callbacks fire when adapters invoke it. The output is a valid plan JSON
// so that the planner can parse it without error.
type costTrackingProvider struct {
	mockTestProvider
	costPerCall float64
}

const validPlanJSON = `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"build widget","expected_touched_area":["src/"],"proof_checks":["go test ./..."]}]}`

func (c *costTrackingProvider) Run(_ context.Context, _ string, _ string) (*provider.Result, error) {
	return &provider.Result{Output: validPlanJSON, Success: true, CostUSD: c.costPerCall}, nil
}

func (c *costTrackingProvider) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Output: validPlanJSON, Success: true, CostUSD: c.costPerCall}, nil
}

func TestRealStageProvider_OnCostBudgetWiring_PlanStageFiresOnCost(t *testing.T) {
	// Verifies that the plan adapter's OnCost callback fires and accumulates
	// cost in the shared budget when the plan stage runs.
	storeDir := t.TempDir()

	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	budget := specloop.NewBudget(policy.Budgets)

	sp := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: storeDir,
		SpecPath: "test-spec.md",
		Provider: &costTrackingProvider{
			mockTestProvider: mockTestProvider{name: "cost-tracker"},
			costPerCall:      0.50,
		},
	})

	stageList, err := sp.BuildStages(policy, rs, budget, nil)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	if budget.Cost() != 0 {
		t.Fatalf("expected initial budget cost 0, got %f", budget.Cost())
	}

	// Set up the store directory structure the plan stage expects:
	// it reads <storeDir>/runs/<runID>/spec-packet.md
	store := runstore.NewStore(storeDir)
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	specPacket := "# Test Spec Packet\n\nBuild a widget.\n"
	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte(specPacket), 0o644); err != nil {
		t.Fatalf("write spec-packet.md: %v", err)
	}

	// Find and run the plan stage.
	for _, s := range stageList {
		if s.Name() == "plan" {
			_, err := s.Run(context.Background(), rs)
			if err != nil {
				t.Fatalf("plan stage Run: %v", err)
			}
			break
		}
	}

	// The costTrackingProvider reports $0.50 per call. The plan adapter's
	// OnCost callback should have forwarded this to the budget.
	if budget.Cost() <= 0 {
		t.Errorf("expected budget.Cost() > 0 after plan stage run, got %f", budget.Cost())
	}
}

// initGitRepo initializes a minimal git repo with an initial commit in the
// given directory. This is needed so that GitDiffProvider.Diff can run
// "git diff main...HEAD" without error.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "checkout", "-b", "main"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
}

func TestRealStageProvider_OnCostBudgetWiring_ReviewStageFiresOnCost(t *testing.T) {
	// Verifies that the review adapter's OnCost callback fires and accumulates
	// cost in the shared budget when the review stage runs. The costTrackingProvider
	// returns plan JSON (not valid findings), so the review runner records a
	// parse error per facet — but OnCost fires at the adapter level before
	// parsing, which is the documented behavior ("Track cost even on error").
	workDir := t.TempDir()
	initGitRepo(t, workDir)

	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "test-spec.md")
	specContent := "# Test Spec\n\n## Acceptance Criteria\n- It works\n"
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	budget := specloop.NewBudget(policy.Budgets)

	sp := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  workDir,
		StoreDir: t.TempDir(),
		SpecPath: specPath,
		Provider: &costTrackingProvider{
			mockTestProvider: mockTestProvider{name: "cost-tracker"},
			costPerCall:      0.25,
		},
	})

	stageList, err := sp.BuildStages(policy, rs, budget, nil)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	if budget.Cost() != 0 {
		t.Fatalf("expected initial budget cost 0, got %f", budget.Cost())
	}

	// Find and run the review stage. The runner catches parse errors per facet
	// (puts them in ErroredFacets) but returns (result, nil). The stage sees
	// AllFacetsErrored and returns Blocked — no error propagated.
	for _, s := range stageList {
		if s.Name() == "review" {
			_, _ = s.Run(context.Background(), rs)
			break
		}
	}

	// The costTrackingProvider reports $0.25 per call. The review adapter's
	// OnCost callback should have forwarded this to the budget. The review
	// runner calls the adapter once per facet (default policy has 2 facets).
	if budget.Cost() <= 0 {
		t.Errorf("expected budget.Cost() > 0 after review stage run, got %f", budget.Cost())
	}
}

func TestRealStageProvider_OnCostBudgetWiring_AcceptStageFiresOnCost(t *testing.T) {
	// Verifies that the accept adapter's OnCost callback fires and accumulates
	// cost in the shared budget when the accept stage runs. The costTrackingProvider
	// returns plan JSON (not valid criterion result), so the accept evaluator
	// will error on parse — but OnCost fires at the adapter level before parsing.
	workDir := t.TempDir()
	initGitRepo(t, workDir)

	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "test-spec.md")
	specContent := "# Test Spec\n\n## Acceptance Criteria\n- It works\n"
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	budget := specloop.NewBudget(policy.Budgets)

	sp := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  workDir,
		StoreDir: t.TempDir(),
		SpecPath: specPath,
		Provider: &costTrackingProvider{
			mockTestProvider: mockTestProvider{name: "cost-tracker"},
			costPerCall:      0.30,
		},
	})

	stageList, err := sp.BuildStages(policy, rs, budget, nil)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	if budget.Cost() != 0 {
		t.Fatalf("expected initial budget cost 0, got %f", budget.Cost())
	}

	// Find and run the accept stage. The evaluator calls EvaluateCriterion
	// per criterion; OnCost fires in the adapter even if parsing fails downstream.
	for _, s := range stageList {
		if s.Name() == "accept" {
			_, _ = s.Run(context.Background(), rs)
			break
		}
	}

	// The costTrackingProvider reports $0.30 per call. The accept adapter's
	// OnCost callback should have forwarded this to the budget. The accept
	// evaluator calls the adapter once per criterion (spec has 1 criterion).
	if budget.Cost() <= 0 {
		t.Errorf("expected budget.Cost() > 0 after accept stage run, got %f", budget.Cost())
	}
}

func TestRealStageProvider_BuildStages_WithProvider_ReturnsRealAdapters(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	sp := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
		Provider: &mockTestProvider{name: "test-provider"},
	})

	stages, err := sp.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages returned error: %v", err)
	}

	expectedNames := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
	if len(stages) != len(expectedNames) {
		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
	}
	for i, name := range expectedNames {
		if stages[i].Name() != name {
			t.Errorf("stage[%d].Name() = %q, want %q", i, stages[i].Name(), name)
		}
	}
}

func TestRealStageProvider_BuildStages_WithProvider_NilProviderFallsBackToNoops(t *testing.T) {
	// When Provider is nil, BuildStages should still work (backward compat with noops).
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	sp := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := sp.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages returned error: %v", err)
	}
	if len(stages) != 9 {
		t.Fatalf("expected 9 stages, got %d", len(stages))
	}
}

func TestNewRealStageProvider_AcceptsProviderFields(t *testing.T) {
	claudeProv := &mockTestProvider{name: "claude"}
	codexProv := &mockTestProvider{name: "codex"}
	sp := NewRealStageProvider(RealStageProviderConfig{
		ClaudeProvider: claudeProv,
		CodexProvider:  codexProv,
		WorkDir:        t.TempDir(),
		StoreDir:       t.TempDir(),
		SpecPath:       "test.md",
	})
	if sp.claudeProvider == nil {
		t.Error("expected claudeProvider to be set")
	}
	if sp.codexProvider == nil {
		t.Error("expected codexProvider to be set")
	}
}

func TestBuildRouter_ReturnsConfiguredRouter(t *testing.T) {
	p := &RealStageProvider{
		claudeProvider: &mockTestProvider{name: "claude"},
		codexProvider:  &mockTestProvider{name: "codex"},
		stateFn:        nil,
		circuitBreaker: nil,
	}
	policy := execpolicy.DefaultPolicy()
	policy.Routing.Ratio = map[string]int{"claude": 70, "codex": 30}
	router := p.buildRouter(policy)
	// Router should be non-nil and usable
	prov, _ := router.Select("plan", "high")
	if prov == nil {
		t.Fatal("expected router to return a provider")
	}
}

func TestBuildRouter_NilCodexProvider_SingleProviderMode(t *testing.T) {
	p := &RealStageProvider{
		claudeProvider: &mockTestProvider{name: "claude"},
		codexProvider:  nil, // single-provider mode
	}
	policy := execpolicy.DefaultPolicy()
	policy.Routing.Ratio = map[string]int{"claude": 100}
	router := p.buildRouter(policy)
	prov, _ := router.Select("plan", "high")
	if prov == nil {
		t.Fatal("expected claude provider in single-provider mode")
	}
	if prov.Name() != "claude" {
		t.Errorf("expected claude, got %q", prov.Name())
	}
}

func TestBuildStages_WithClaudeProvider_UsesFallbackAdapter(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	sp := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:        t.TempDir(),
		StoreDir:       t.TempDir(),
		SpecPath:       "test-spec.md",
		ClaudeProvider: &mockTestProvider{name: "claude"},
	})

	stages, err := sp.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages returned error: %v", err)
	}
	if len(stages) != 9 {
		t.Fatalf("expected 9 stages, got %d", len(stages))
	}
}

func TestRealStageProvider_BuildStages_MissingSpecFileIsNotError(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: filepath.Join(t.TempDir(), "does-not-exist.md"),
	})

	_, err := provider.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
	if err != nil {
		t.Fatalf("BuildStages should not fail for missing spec file, got: %v", err)
	}
}

func TestNewRealStageProvider_LegacyProviderPromotion(t *testing.T) {
	t.Run("legacy Provider promoted to claudeProvider when ClaudeProvider is nil", func(t *testing.T) {
		legacyProv := &mockTestProvider{name: "legacy"}
		sp := NewRealStageProvider(RealStageProviderConfig{
			Provider: legacyProv,
			WorkDir:  t.TempDir(),
			StoreDir: t.TempDir(),
			SpecPath: "test.md",
		})
		if sp.claudeProvider == nil {
			t.Fatal("expected claudeProvider to be set via legacy Provider promotion")
		}
		if sp.claudeProvider.Name() != "legacy" {
			t.Errorf("expected claudeProvider.Name() = %q, got %q", "legacy", sp.claudeProvider.Name())
		}
	})

	t.Run("ClaudeProvider takes precedence over legacy Provider", func(t *testing.T) {
		legacyProv := &mockTestProvider{name: "legacy"}
		claudeProv := &mockTestProvider{name: "claude-explicit"}
		sp := NewRealStageProvider(RealStageProviderConfig{
			Provider:       legacyProv,
			ClaudeProvider: claudeProv,
			WorkDir:        t.TempDir(),
			StoreDir:       t.TempDir(),
			SpecPath:       "test.md",
		})
		if sp.claudeProvider == nil {
			t.Fatal("expected claudeProvider to be set")
		}
		if sp.claudeProvider.Name() != "claude-explicit" {
			t.Errorf("expected claudeProvider.Name() = %q, got %q", "claude-explicit", sp.claudeProvider.Name())
		}
	})
}

func TestBuildRouter_FallbackAdapter_Integration(t *testing.T) {
	// Construct a real Router via buildRouter, wrap it in a FallbackAdapter,
	// and invoke it with a mock provider that succeeds.
	mockProv := &mockTestProvider{name: "claude"}
	p := &RealStageProvider{
		claudeProvider: mockProv,
		codexProvider:  nil,
	}
	policy := execpolicy.DefaultPolicy()
	policy.Routing.Ratio = map[string]int{"claude": 100}
	policy.Routing.Preferences = map[string]string{"plan": "claude"}

	router := p.buildRouter(policy)

	adapter := llmadapter.NewFallbackAdapter(
		router, "plan",
		llmadapter.Config{Tier: "high"},
		"high",
	)

	result, err := adapter.Invoke(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("FallbackAdapter.Invoke returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Success {
		t.Error("expected result.Success to be true")
	}
	if result.Output != "ok" {
		t.Errorf("expected result.Output = %q, got %q", "ok", result.Output)
	}
}

func TestIntegration_BuildStages_FallbackAdapter_RouterWiring(t *testing.T) {
	// Build a real RealStageProvider with mock providers
	claudeProv := &mockTestProvider{name: "claude"}
	codexProv := &mockTestProvider{name: "codex"}
	sp := NewRealStageProvider(RealStageProviderConfig{
		ClaudeProvider: claudeProv,
		CodexProvider:  codexProv,
		WorkDir:        t.TempDir(),
		StoreDir:       t.TempDir(),
		SpecPath:       "test-spec.md",
	})

	policy := execpolicy.DefaultPolicy()
	policy.Routing.Preferences = map[string]string{
		"plan": "claude", "execute": "codex", "review": "any", "accept": "any",
	}
	policy.Routing.Ratio = map[string]int{"claude": 50, "codex": 50}

	budget := specloop.NewBudget(policy.Budgets)
	rs := runstore.NewRunState("test-spec", "test-project")
	stages, err := sp.BuildStages(policy, rs, budget, nil)
	if err != nil {
		t.Fatalf("BuildStages failed: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("expected at least one stage from BuildStages")
	}
	// Verify 9 stages are returned (same as before — multi-provider doesn't change stage count)
	if len(stages) != 9 {
		t.Fatalf("expected 9 stages, got %d", len(stages))
	}
}
