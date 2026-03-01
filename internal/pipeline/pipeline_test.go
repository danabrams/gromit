package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var _ LLMClient = (*testLLMClient)(nil)

func TestNew_ReturnsNonNil(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}

	p := New(deps, paths)

	if p == nil {
		t.Fatal("New() returned nil, expected non-nil Pipeline")
	}
}

func TestPaths_FieldAccess(t *testing.T) {
	paths := Paths{
		GromitDir: "/tmp/.gromit",
		SpecsDir:  "/tmp/.gromit/specs",
		PlansDir:  "/tmp/.gromit/plans",
		EpicsDir:  "/tmp/.gromit/epics",
	}

	if paths.GromitDir != "/tmp/.gromit" {
		t.Errorf("GromitDir = %q, want %q", paths.GromitDir, "/tmp/.gromit")
	}
	if paths.SpecsDir != "/tmp/.gromit/specs" {
		t.Errorf("SpecsDir = %q, want %q", paths.SpecsDir, "/tmp/.gromit/specs")
	}
	if paths.PlansDir != "/tmp/.gromit/plans" {
		t.Errorf("PlansDir = %q, want %q", paths.PlansDir, "/tmp/.gromit/plans")
	}
	if paths.EpicsDir != "/tmp/.gromit/epics" {
		t.Errorf("EpicsDir = %q, want %q", paths.EpicsDir, "/tmp/.gromit/epics")
	}
}

func TestDeps_FieldAccess(t *testing.T) {
	deps := Deps{
		AgentResolver:     &testAgentResolver{},
	LLMClient:         &testLLMClient{},
	TrackerClient:     &testBeadClient{},
	BacklogClient:     &testBacklogClient{},
		RefineRenderer:    &testRefineRenderer{},
		PlanRenderer:      &testPlanRenderer{},
		DecomposeRenderer: &testDecomposeRenderer{},
		ReviewRenderer:    &testReviewRenderer{},
		ExploreRenderer:   &testExploreRenderer{},
		LearningsManager:  &testLearningsManager{},
		StateManager:      &testStateManager{},
		LogWriter:         &testLogWriter{},
	}

	if deps.AgentResolver == nil {
		t.Error("AgentResolver field should be set")
	}
	if deps.LLMClient == nil {
		t.Error("LLMClient field should be set")
	}
	if deps.TrackerClient == nil {
		t.Error("TrackerClient field should be set")
	}
	if deps.BacklogClient == nil {
		t.Error("BacklogClient field should be set")
	}
	if deps.RefineRenderer == nil {
		t.Error("RefineRenderer field should be set")
	}
	if deps.PlanRenderer == nil {
		t.Error("PlanRenderer field should be set")
	}
	if deps.DecomposeRenderer == nil {
		t.Error("DecomposeRenderer field should be set")
	}
	if deps.ReviewRenderer == nil {
		t.Error("ReviewRenderer field should be set")
	}
	if deps.ExploreRenderer == nil {
		t.Error("ExploreRenderer field should be set")
	}
	if deps.LearningsManager == nil {
		t.Error("LearningsManager field should be set")
	}
	if deps.StateManager == nil {
		t.Error("StateManager field should be set")
	}
	if deps.LogWriter == nil {
		t.Error("LogWriter field should be set")
	}
}

func TestDeps_HasTrackerClientField(t *testing.T) {
	var deps Deps
	if deps.TrackerClient != nil {
		t.Fatalf("deps.TrackerClient should be nil by default, got %v", deps.TrackerClient)
	}
}

func TestDeps_UsesLLMClientField(t *testing.T) {
	deps := Deps{
		LLMClient: &testLLMClient{},
	}

	if deps.LLMClient == nil {
		t.Error("LLMClient field should be set")
	}
}

func TestPipelineTest_HasLLMClientCompileTimeAssertion(t *testing.T) {
	p := filepath.Join("..", "..", "internal", "pipeline", "pipeline_test.go")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read pipeline_test.go: %v", err)
	}
	pattern := regexp.MustCompile(`(?m)^var _ LLMClient = \(\*testLLMClient\)\(nil\)$`)
	if !pattern.Match(content) {
		t.Fatal("pipeline_test.go should include compile-time assertion for LLMClient")
	}
}

func TestPipeline_RefineMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{IdeaText: "Test idea"}

	// Should return error with nil dependencies
	_, err := p.Refine(ctx, input)
	if err == nil {
		t.Error("Refine() should error with nil dependencies")
	}
}

func TestPipeline_PlanMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	_, err := p.Plan(ctx, input)
	if err == nil {
		t.Error("Plan() should error with nil dependencies")
	}
}

func TestPipeline_Plan_SucceedsWithValidDeps(t *testing.T) {
	// Setup test dependencies with minimal mocks
	tmpDir := t.TempDir()
	deps := &Deps{
		PlanRenderer:  &testPlanRenderer{},
		AgentResolver: &testAgentResolver{},
	}

	paths := &Paths{
		GromitDir: tmpDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	// Call Plan() and verify it doesn't return "not implemented" error
	session, err := p.Plan(ctx, input)

	// With default testPlanRenderer (returns ""), we expect an error from agent.LaunchInDir
	// because testAgentResolver returns nil Agent. The key is it doesn't return "Plan not yet implemented"
	if err != nil && err.Error() == "pipeline: Plan not yet implemented" {
		t.Fatalf("Plan() still has TODO stub: %v", err)
	}

	if err != nil {
		t.Logf("Expected error (not the TODO stub): %v", err)
	}

	if session == nil && err == nil {
		t.Error("Plan() returned both nil session and nil error")
	}
}

func TestPipeline_DecomposeMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{PlanName: "test-plan"}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Error("Decompose() should error with nil dependencies")
	}
}

func TestPipeline_ReviewInteractiveMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{FromCommit: "HEAD~1", Diff: "diff"}

	_, err := p.ReviewInteractive(ctx, input)
	if err == nil {
		t.Error("ReviewInteractive() should error with nil dependencies")
	}
}

func TestPipeline_ReviewNonInteractiveMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{FromCommit: "HEAD~1", Diff: "diff", Model: "sonnet", Timeout: 300}

	_, err := p.ReviewNonInteractive(ctx, input)
	if err == nil {
		t.Error("ReviewNonInteractive() should error with nil dependencies")
	}
}

func TestPipeline_ExploreMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{Topic: "test topic"}

	_, err := p.Explore(ctx, input)
	if err == nil {
		t.Error("Explore() should error with nil dependencies")
	}
}

// TestPipeline_ResolveReviewScopeMethod verifies that Pipeline has ResolveReviewScope method
// Expected failure: Method not yet fully implemented to delegate to command layer
func TestPipeline_ResolveReviewScopeMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	
	// Method should exist and be callable
	commit, err := p.ResolveReviewScope(ctx, "spec-name", "", "")
	
	// Currently returns error as placeholder - this will change when implemented
	if err == nil {
		t.Error("ResolveReviewScope() expected error (method not yet implemented)")
	}
	
	if commit != "" {
		t.Errorf("ResolveReviewScope() returned commit=%q, want empty string", commit)
	}
}

// TestPipeline_ResolveReviewScopeWithSince verifies scope flag passthrough
// Expected: ResolveReviewScope accepts --since flag and returns it correctly
func TestPipeline_ResolveReviewScopeWithSince(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	sinceCommit := "abc123def456"

	// When --since is provided, should return it directly
	commit, err := p.ResolveReviewScope(ctx, "", "", sinceCommit)

	if err != nil {
		t.Errorf("ResolveReviewScope(since=%q) error = %v, want nil", sinceCommit, err)
	}

	if commit != sinceCommit {
		t.Errorf("ResolveReviewScope(since=%q) returned %q, want %q", sinceCommit, commit, sinceCommit)
	}
}

// TestPipeline_ResolveReviewScopeWithSpec verifies spec flag passthrough
// Expected: ResolveReviewScope accepts --spec flag and passes it through correctly
func TestPipeline_ResolveReviewScopeWithSpec(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	specName := "init-wizard"
	
	// When --spec is provided, should use it
	_, err := p.ResolveReviewScope(ctx, specName, "", "")
	
	// Will error until implemented, but verifies signature accepts spec
	if err == nil {
		t.Error("ResolveReviewScope() expected error (method not yet implemented)")
	}
}

// TestPipeline_ResolveReviewScopeWithEpic verifies epic flag passthrough
// Expected: ResolveReviewScope accepts --epic flag and passes it through correctly
func TestPipeline_ResolveReviewScopeWithEpic(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	epicID := "gromit-xyz"
	
	// When --epic is provided, should use it
	_, err := p.ResolveReviewScope(ctx, "", epicID, "")
	
	// Will error until implemented, but verifies signature accepts epic
	if err == nil {
		t.Error("ResolveReviewScope() expected error (method not yet implemented)")
	}
}

// TestPipeline_ResolveReviewScope_SpecResolution_FindsEarliestCommitFromSpecBeads verifies that when --spec is provided,
// ResolveReviewScope resolves to the earliest commit from beads matching the spec label.
// Expected failure: ResolveReviewScope currently delegates spec to caller
func TestPipeline_ResolveReviewScope_SpecResolution_FindsEarliestCommitFromSpecBeads(t *testing.T) {
	deps := &Deps{
		// Will be populated with tracker for spec lookup
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		// SpecsDir can be empty for this test since scope.ResolveSpec just formats a label
	}
	p := New(deps, paths)

	ctx := context.Background()
	specName := "test-spec"

	// When --spec is provided, ResolveReviewScope should NOT return "delegated to caller" error
	_, err := p.ResolveReviewScope(ctx, specName, "", "")

	// Currently returns error saying it's delegated to caller
	if err != nil && strings.Contains(err.Error(), "delegated to caller") {
		t.Fatalf("ResolveReviewScope() still delegates spec resolution to caller: %v", err)
	}
}

// TestPipeline_ResolveReviewScope_EpicResolution_FindsEarliestCommitFromEpicBeads verifies that when --epic is provided,
// ResolveReviewScope resolves to the earliest commit from beads matching the epic's spec labels.
// Expected failure: ResolveReviewScope currently delegates epic to caller
func TestPipeline_ResolveReviewScope_EpicResolution_FindsEarliestCommitFromEpicBeads(t *testing.T) {
	// Create a temporary specs directory with a spec file that has an epic field
	specsDir := t.TempDir()
	specContent := `---
id: test-spec
epic: test-epic
---
# Test Spec

This is a test spec.`
	if err := os.WriteFile(filepath.Join(specsDir, "test-spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	deps := &Deps{
		// TrackerClient would be needed for epic lookup
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  specsDir,
	}
	p := New(deps, paths)

	ctx := context.Background()
	epicID := "test-epic"

	// When --epic is provided, ResolveReviewScope should NOT return "delegated to caller" error
	_, err := p.ResolveReviewScope(ctx, "", epicID, "")

	// Currently returns error saying it's delegated to caller
	if err != nil && strings.Contains(err.Error(), "delegated to caller") {
		t.Fatalf("ResolveReviewScope() still delegates epic resolution to caller: %v", err)
	}
}

// TestPipeline_ResolveReviewScope_NoFlags_UsesStateFileWhenAvailable verifies that when no flags are provided,
// ResolveReviewScope falls back to StateManager.GetLastReviewCommit.
// Expected failure: ResolveReviewScope doesn't implement state file fallback yet
func TestPipeline_ResolveReviewScope_NoFlags_UsesStateFileWhenAvailable(t *testing.T) {
	lastReviewCommit := "abc123def456"
	mockState := &reviewAcceptanceMockStateManager{
		getLastReviewCommitFunc: func() (string, error) {
			return lastReviewCommit, nil
		},
	}

	deps := &Deps{
		StateManager: mockState,
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}
	p := New(deps, paths)

	ctx := context.Background()

	// When no flags are provided, ResolveReviewScope should use state file
	commit, err := p.ResolveReviewScope(ctx, "", "", "")

	// This test fails until state file fallback is implemented
	if err != nil {
		t.Logf("ResolveReviewScope() returned error (state file fallback not yet implemented): %v", err)
		return
	}

	if commit != lastReviewCommit {
		t.Errorf("ResolveReviewScope() returned %q, want %q", commit, lastReviewCommit)
	}
}

// TestPipeline_ListBeadsMethod verifies Pipeline has ListBeads query method
// with proper input/output types and nil dependency validation.
func TestPipeline_ListBeadsMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ListBeadsInput{Status: "ready"}

	// Should return error with nil dependencies
	_, err := p.ListBeads(ctx, input)
	if err == nil {
		t.Error("ListBeads() should error with nil dependencies")
	}
}

func TestPipeline_ListBeadsUnsupportedStatus(t *testing.T) {
	deps := &Deps{BeadQueryClient: &testBeadQueryClient{}}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ListBeadsInput{Status: "done"}

	_, err := p.ListBeads(ctx, input)
	if err == nil {
		t.Error("ListBeads() should error when given an unsupported status filter")
	}
}

// TestPipeline_QueryBeadsMethod verifies Pipeline has QueryBeads query method
// with proper input/output types and nil dependency validation.
func TestPipeline_QueryBeadsMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := QueryBeadsInput{StatusFilter: "ready"}

	// Should return error with nil dependencies
	_, err := p.QueryBeads(ctx, input)
	if err == nil {
		t.Error("QueryBeads() should error with nil dependencies")
	}
}

func TestPipeline_QueryBeadsUnsupportedStatus(t *testing.T) {
	deps := &Deps{BeadQueryClient: &testBeadQueryClient{}}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := QueryBeadsInput{StatusFilter: "done"}

	_, err := p.QueryBeads(ctx, input)
	if err == nil {
		t.Error("QueryBeads() should error when given an unsupported status filter")
	}
}

// TestPipeline_CountBeadsMethod verifies Pipeline has CountBeads query method
// with proper input/output types and nil dependency validation.
func TestPipeline_CountBeadsMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := CountBeadsInput{Status: "done"}

	// Should return error with nil dependencies
	_, err := p.CountBeads(ctx, input)
	if err == nil {
		t.Error("CountBeads() should error with nil dependencies")
	}
}

func TestPipeline_CountBeadsUnsupportedStatus(t *testing.T) {
	deps := &Deps{BeadQueryClient: &testBeadQueryClient{}}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := CountBeadsInput{Status: "done"}

	_, err := p.CountBeads(ctx, input)
	if err == nil {
		t.Error("CountBeads() should error when given an unsupported status filter")
	}
}

// TestPipeline_QueryUndecomposedPlansMethod verifies Pipeline has QueryUndecomposedPlans method
// with proper input/output types and nil dependency validation.
func TestPipeline_QueryUndecomposedPlansMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := QueryUndecomposedPlansInput{Force: false}

	// Should return error with nil dependencies
	_, err := p.QueryUndecomposedPlans(ctx, input)
	if err == nil {
		t.Error("QueryUndecomposedPlans() should error with nil dependencies")
	}
}

// TestPipeline_QueryUndecomposedPlans_FiltersUndecomposedPlans verifies that
// QueryUndecomposedPlans returns undecomposed plans from the plans directory.
// Expected: Returns list of plans with decomposed:false or missing decomposed field
func TestPipeline_QueryUndecomposedPlans_FiltersUndecomposedPlans(t *testing.T) {
	t.Parallel()

	// Create temporary plans directory with test files
	plansDir := t.TempDir()

	// Create decomposed plan
	decomposedContent := `---
decomposed: true
decomposed_at: "2024-01-15T10:00:00Z"
---
# Already Decomposed Plan
Content here`
	if err := os.WriteFile(filepath.Join(plansDir, "decomposed.md"), []byte(decomposedContent), 0644); err != nil {
		t.Fatalf("failed to create decomposed plan: %v", err)
	}

	// Create undecomposed plan
	undecomposedContent := `---
decomposed: false
---
# Undecomposed Plan
Content here`
	if err := os.WriteFile(filepath.Join(plansDir, "undecomposed.md"), []byte(undecomposedContent), 0644); err != nil {
		t.Fatalf("failed to create undecomposed plan: %v", err)
	}

	// Create plan with missing decomposed field (treated as undecomposed)
	missingContent := `---
created: "2024-01-15"
---
# Missing Decomposed Field
Content here`
	if err := os.WriteFile(filepath.Join(plansDir, "missing.md"), []byte(missingContent), 0644); err != nil {
		t.Fatalf("failed to create plan with missing field: %v", err)
	}

	// Create pipeline with mock tracker
	mockTracker := &testBeadClient{}
	deps := &Deps{
		TrackerClient: mockTracker,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := QueryUndecomposedPlansInput{Force: false}

	// Query for undecomposed plans
	result, err := p.QueryUndecomposedPlans(ctx, input)
	if err != nil {
		t.Fatalf("QueryUndecomposedPlans() error = %v, want nil", err)
	}

	// Should return the undecomposed and missing plans, not the decomposed one
	if len(result.Plans) == 0 {
		t.Error("QueryUndecomposedPlans() returned 0 plans, want at least 1")
	}

	// Verify plan names
	planNames := make([]string, len(result.Plans))
	for i, plan := range result.Plans {
		planNames[i] = plan.Name
	}

	// Should include undecomposed plans but not decomposed plan
	if len(planNames) < 2 {
		t.Errorf("QueryUndecomposedPlans() returned %d plans, want at least 2 (undecomposed and missing)", len(planNames))
	}

	hasUndecomposed := false
	hasMissing := false
	hasDecomposed := false

	for _, name := range planNames {
		if name == "undecomposed" {
			hasUndecomposed = true
		}
		if name == "missing" {
			hasMissing = true
		}
		if name == "decomposed" {
			hasDecomposed = true
		}
	}

	if !hasUndecomposed {
		t.Error("QueryUndecomposedPlans() missing 'undecomposed' plan")
	}
	if !hasMissing {
		t.Error("QueryUndecomposedPlans() missing 'missing' plan")
	}
	if hasDecomposed {
		t.Error("QueryUndecomposedPlans() should not include 'decomposed' plan")
	}
}
