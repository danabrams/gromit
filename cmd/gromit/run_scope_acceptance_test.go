package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/spf13/cobra"
)

// TestRunCommand_SpecFlagFiltersBeads is an acceptance test verifying that
// `gromit run --spec <name>` only processes beads with the matching spec label.
func TestRunCommand_SpecFlagFiltersBeads(t *testing.T) {
	// This test verifies the end-to-end flow of --spec flag:
	// 1. Parse --spec flag from command line
	// 2. Call scope.ResolveSpec to get label
	// 3. Pass label to runner for filtering

	// Set up flag values as they would be from CLI
	specFlag := "init-wizard"
	epicFlag := ""

	// Verify scope.ValidateFlags accepts this combination
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --spec alone, got error: %v", err)
	}

	// Verify scope.ResolveSpec returns correct label
	labels := scope.ResolveSpec(specFlag)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Fatalf("ResolveSpec returned %q, want %q", labels[0], expectedLabel)
	}

	// TODO: This test will fully pass when runLoop() accepts and uses --spec flag
	// For now, we verify the scope package functions work correctly
	t.Skip("Pending integration: runLoop() does not yet accept --spec flag and pass labels to runner")
}

// TestRunCommand_EpicFlagFiltersBeads is an acceptance test verifying that
// `gromit run --epic <id>` resolves to specs for that epic and only processes their beads.
func TestRunCommand_EpicFlagFiltersBeads(t *testing.T) {
	// This test verifies the end-to-end flow of --epic flag:
	// 1. Parse --epic flag from command line
	// 2. Call scope.ResolveEpic to get labels for all specs in that epic
	// 3. Pass labels to runner for filtering

	// Create temp directory with spec files
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec files linked to the epic
	specs := []struct {
		filename string
		id       string
		epic     string
	}{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
		{"settings.md", "settings", "other-epic"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := fmt.Sprintf(`---
id: %s
epic: %s
created: 2026-02-08
---

# Spec
`, spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	// Set up flag values
	epicFlag := "gromit-xyz"
	specFlag := ""

	// Verify scope.ValidateFlags accepts this combination
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic alone, got error: %v", err)
	}

	// Verify scope.ResolveEpic returns correct labels
	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels for gromit-xyz, got %d: %v", len(labels), labels)
	}

	// Verify labels are correct
	expectedLabels := map[string]bool{"spec:auth": false, "spec:profile": false}
	for _, label := range labels {
		if _, exists := expectedLabels[label]; !exists {
			t.Errorf("Unexpected label %q", label)
		}
		expectedLabels[label] = true
	}
	for label, found := range expectedLabels {
		if !found {
			t.Errorf("Missing expected label %q", label)
		}
	}

	// TODO: This test will fully pass when runLoop() accepts and uses --epic flag
	t.Skip("Pending integration: runLoop() does not yet accept --epic flag and pass labels to runner")
}

// TestRunCommand_EpicAndSpecFlagsMutuallyExclusive is an acceptance test verifying that
// providing both --epic and --spec flags results in an error.
func TestRunCommand_EpicAndSpecFlagsMutuallyExclusive(t *testing.T) {
	// This test verifies that scope.ValidateFlags correctly rejects both flags being set

	epicFlag := "gromit-xyz"
	specFlag := "init-wizard"

	// Verify scope.ValidateFlags rejects this combination
	err := scope.ValidateFlags(epicFlag, specFlag)
	if err == nil {
		t.Fatal("ValidateFlags should return error when both --epic and --spec are provided")
	}

	// Verify error message is clear
	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "mutually exclusive") {
		t.Errorf("Error message should mention 'mutually exclusive', got: %q", errMsg)
	}

	// TODO: This test will fully pass when runLoop() calls scope.ValidateFlags and returns error early
	t.Skip("Pending integration: runLoop() does not yet call scope.ValidateFlags before processing")
}

// TestRunCommand_NoScopeFlagUsesDefaultBehavior is an acceptance test verifying that
// when neither --epic nor --spec is provided, all beads are processed (default behavior).
func TestRunCommand_NoScopeFlagUsesDefaultBehavior(t *testing.T) {
	// This test verifies that when no scope flags are set, validation passes
	// and no labels are passed to the runner (default behavior)

	epicFlag := ""
	specFlag := ""

	// Verify scope.ValidateFlags accepts empty flags
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept both flags empty, got error: %v", err)
	}

	// When both flags are empty, no labels should be passed to runner
	// The runner should use its default Ready() method without label filtering

	// TODO: This test will fully pass when we verify runLoop() behavior with no flags
	t.Skip("Pending integration: need to verify runLoop() preserves default behavior when flags are empty")
}

// TestRunCommand_SpecFlagWithOtherFlags is an acceptance test verifying that
// --spec works correctly with existing flags like -n and --time-budget.
func TestRunCommand_SpecFlagWithOtherFlags(t *testing.T) {
	// This test verifies that --spec flag can be combined with other run command flags

	specFlag := "init-wizard"
	epicFlag := ""

	// Verify scope.ValidateFlags accepts --spec flag
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --spec flag, got error: %v", err)
	}

	// Verify scope.ResolveSpec returns label
	labels := scope.ResolveSpec(specFlag)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}

	// Expected behavior:
	// - The labels from ResolveSpec should be passed to runner
	// - Runner should respect other flags like -n (maxIterations) and --time-budget
	// - The combination should work: filter by label AND apply iteration/time limits

	t.Skip("Pending integration: need to verify runLoop() combines --spec with -n and --time-budget flags")
}

// TestRunCommand_EpicFlagWithOtherFlags is an acceptance test verifying that
// --epic works correctly with existing flags like -n and --time-budget.
func TestRunCommand_EpicFlagWithOtherFlags(t *testing.T) {
	// Create temp directory with spec files
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec file linked to epic
	specPath := filepath.Join(specsDir, "auth.md")
	specContent := `---
id: auth
epic: gromit-xyz
created: 2026-02-08
---

# Auth Spec
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	epicFlag := "gromit-xyz"
	specFlag := ""

	// Verify scope.ValidateFlags accepts --epic flag
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic flag, got error: %v", err)
	}

	// Verify scope.ResolveEpic returns labels
	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("ResolveEpic should return 1 label, got %d", len(labels))
	}

	// Expected behavior:
	// - The labels from ResolveEpic should be passed to runner
	// - Runner should respect other flags like -n (maxIterations) and --time-budget
	// - The combination should work: filter by labels AND apply iteration/time limits

	t.Skip("Pending integration: need to verify runLoop() combines --epic with -n and --time-budget flags")
}

// TestRunCommand_SpecFlagWithNonexistentSpec is an acceptance test verifying behavior
// when --spec is provided with a spec name that has no matching beads.
func TestRunCommand_SpecFlagWithNonexistentSpec(t *testing.T) {
	// This test verifies that using --spec with a nonexistent spec is gracefully handled

	specFlag := "nonexistent-spec"
	epicFlag := ""

	// Verify scope.ValidateFlags accepts the flag
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --spec flag, got error: %v", err)
	}

	// Verify scope.ResolveSpec returns a label (even if no beads match it)
	labels := scope.ResolveSpec(specFlag)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	expectedLabel := "spec:nonexistent-spec"
	if labels[0] != expectedLabel {
		t.Fatalf("ResolveSpec returned %q, want %q", labels[0], expectedLabel)
	}

	// Expected behavior:
	// - When runner queries bd with this label, it should get no beads
	// - Runner should exit cleanly with "no beads ready" message
	// - No error should occur

	t.Skip("Pending integration: need to verify runLoop() handles no matching beads gracefully")
}

// TestRunCommand_EpicFlagWithNonexistentEpic is an acceptance test verifying behavior
// when --epic is provided with an epic ID that has no linked specs.
func TestRunCommand_EpicFlagWithNonexistentEpic(t *testing.T) {
	// Create temp directory with spec files
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create a spec linked to a different epic
	specPath := filepath.Join(specsDir, "auth.md")
	specContent := `---
id: auth
epic: other-epic
created: 2026-02-08
---

# Auth Spec
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	epicFlag := "nonexistent-epic"
	specFlag := ""

	// Verify scope.ValidateFlags accepts the flag
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic flag, got error: %v", err)
	}

	// Verify scope.ResolveEpic returns empty labels (no specs match)
	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic should not error for nonexistent epic, got error: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("ResolveEpic should return 0 labels for nonexistent epic, got %d: %v", len(labels), labels)
	}

	// Expected behavior:
	// - When labels slice is empty, runner should have no beads to process
	// - Runner should exit cleanly with "no beads ready" or similar message
	// - No error should occur

	t.Skip("Pending integration: need to verify runLoop() handles empty label list gracefully")
}

// TestRunCommand_SpecFlagPassedToRunner is an acceptance test verifying that
// the --spec flag value is correctly passed to the runner for label filtering.
func TestRunCommand_SpecFlagPassedToRunner(t *testing.T) {
	// This test verifies the full integration chain from CLI flag to runner

	// Step 1: Verify command has --spec flag defined
	// (This will fail to compile if flag doesn't exist)
	cmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flag values
			specFlag, err := cmd.Flags().GetString("spec")
			if err != nil {
				return fmt.Errorf("--spec flag not found: %w", err)
			}
			epicFlag, err := cmd.Flags().GetString("epic")
			if err != nil {
				return fmt.Errorf("--epic flag not found: %w", err)
			}

			// Validate flags
			if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
				return err
			}

			// Resolve spec to labels
			if specFlag != "" {
				labels := scope.ResolveSpec(specFlag)
				// TODO: Pass labels to runner
				_ = labels
			}

			return nil
		},
	}
	cmd.Flags().String("spec", "", "Filter beads by spec label")
	cmd.Flags().String("epic", "", "Filter beads by epic")

	// Step 2: Simulate running with --spec flag
	cmd.SetArgs([]string{"--spec", "init-wizard"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// This test demonstrates the expected integration but doesn't verify
	// the actual runCmd in main.go yet
	t.Skip("Pending integration: actual runCmd in main.go doesn't have --spec flag yet")
}

// TestRunCommand_EpicFlagPassedToRunner is an acceptance test verifying that
// the --epic flag value is correctly passed to the runner for label filtering.
func TestRunCommand_EpicFlagPassedToRunner(t *testing.T) {
	// Create temp directory with spec files
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec file linked to epic
	specPath := filepath.Join(specsDir, "auth.md")
	specContent := `---
id: auth
epic: gromit-xyz
created: 2026-02-08
---

# Auth Spec
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// This test verifies the full integration chain from CLI flag to runner

	// Step 1: Verify command has --epic flag defined
	cmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flag values
			epicFlag, err := cmd.Flags().GetString("epic")
			if err != nil {
				return fmt.Errorf("--epic flag not found: %w", err)
			}
			specFlag, err := cmd.Flags().GetString("spec")
			if err != nil {
				return fmt.Errorf("--spec flag not found: %w", err)
			}

			// Validate flags
			if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
				return err
			}

			// Resolve epic to labels
			if epicFlag != "" {
				labels, err := scope.ResolveEpic(epicFlag, specsDir)
				if err != nil {
					return err
				}
				// TODO: Pass labels to runner
				_ = labels
			}

			return nil
		},
	}
	cmd.Flags().String("epic", "", "Filter beads by epic")
	cmd.Flags().String("spec", "", "Filter beads by spec label")

	// Step 2: Simulate running with --epic flag
	cmd.SetArgs([]string{"--epic", "gromit-xyz"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// This test demonstrates the expected integration but doesn't verify
	// the actual runCmd in main.go yet
	t.Skip("Pending integration: actual runCmd in main.go doesn't have --epic flag yet")
}

// TestRunCommand_ScopeValidationCalledInRunLoop is an acceptance test verifying that
// scope.ValidateFlags is called in runLoop() to enforce mutual exclusivity.
func TestRunCommand_ScopeValidationCalledInRunLoop(t *testing.T) {
	// This test verifies that validation happens before runner creation

	// Step 1: Create a command that mimics runLoop validation logic
	cmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flag values (same order as runLoop would)
			epicFlag, _ := cmd.Flags().GetString("epic")
			specFlag, _ := cmd.Flags().GetString("spec")

			// Validate flags BEFORE creating runner (early exit pattern)
			if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
				return err // Return error immediately, don't create runner
			}

			// If validation passes, would continue to create runner...
			return nil
		},
	}
	cmd.Flags().String("epic", "", "Filter beads by epic")
	cmd.Flags().String("spec", "", "Filter beads by spec label")

	// Step 2: Try to run with both flags
	cmd.SetArgs([]string{"--epic", "gromit-xyz", "--spec", "init-wizard"})
	err := cmd.Execute()

	// Step 3: Verify it returns validation error
	if err == nil {
		t.Fatal("Command should return validation error when both flags are provided")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("Error should mention mutual exclusivity, got: %v", err)
	}

	// This test demonstrates the expected flow but doesn't verify
	// the actual runLoop() function yet
	t.Skip("Pending integration: actual runLoop() in main.go doesn't call scope.ValidateFlags yet")
}

// TestRunnerWithLabels_CallsReadyWithLabel is an acceptance test verifying that
// when the runner is given label filters, it calls ReadyWithLabel instead of Ready.
func TestRunnerWithLabels_CallsReadyWithLabel(t *testing.T) {
	// This test verifies that the Runner, when configured with label filters,
	// uses the BeadClient.ReadyWithLabel method instead of BeadClient.Ready.

	// Create a mock BeadClient that tracks which methods are called
	mockBeads := &mockBeadClientForLabelTest{
		readyCalled:          false,
		readyWithLabelCalled: false,
		labelUsed:            "",
	}

	// Create a minimal config
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P1:         "sonnet",
			Validation: "haiku",
		},
		Loop: config.LoopConfig{
			MaxIterations: 1, // Only run once
		},
		Paths: config.PathsConfig{
			Templates:       t.TempDir(),
			Specs:           t.TempDir(),
			Logs:            filepath.Join(t.TempDir(), "logs"),
			ProjectClaudeMD: filepath.Join(t.TempDir(), "CLAUDE.md"),
		},
	}

	// Create a runner with mock dependencies
	r, err := runner.NewRunnerWithDeps(cfg, &strings.Builder{}, t.TempDir(), runner.Deps{
		Beads:    mockBeads,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockAnalyzer{},
		Renderer: &mockRenderer{},
		Logger:   &mockLogger{},
	})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Run with label filters
	ctx := context.Background()
	// TODO: This call signature doesn't exist yet - we need to add labels parameter
	// err = r.Run(ctx, 1, time.Time{}, false, []string{"spec:init-wizard"})
	// For now, just call Run() without labels to verify the test infrastructure works
	_ = r
	_ = ctx

	// When the feature is implemented, verify that ReadyWithLabel was called
	// if mockBeads.readyCalled {
	//     t.Error("Runner called Ready() when labels were provided, should call ReadyWithLabel()")
	// }
	// if !mockBeads.readyWithLabelCalled {
	//     t.Error("Runner did not call ReadyWithLabel() when labels were provided")
	// }
	// if mockBeads.labelUsed != "spec:init-wizard" {
	//     t.Errorf("Runner called ReadyWithLabel with %q, want %q", mockBeads.labelUsed, "spec:init-wizard")
	// }

	t.Skip("Pending implementation: Runner.Run does not yet accept label filters parameter")
}

// Mock implementations for testing

type mockBeadClientForLabelTest struct {
	readyCalled          bool
	readyWithLabelCalled bool
	labelUsed            string
}

func (m *mockBeadClientForLabelTest) Ready() (*bead.Bead, error) {
	m.readyCalled = true
	return nil, nil // No beads ready
}

func (m *mockBeadClientForLabelTest) ReadyWithLabel(label string) (*bead.Bead, error) {
	m.readyWithLabelCalled = true
	m.labelUsed = label
	return nil, nil // No beads ready
}

func (m *mockBeadClientForLabelTest) ListWithLabel(label string) ([]*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForLabelTest) Show(id string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForLabelTest) Close(id string) error {
	return nil
}

func (m *mockBeadClientForLabelTest) Sync() error {
	return nil
}

func (m *mockBeadClientForLabelTest) AddComment(id, comment string) error {
	return nil
}

func (m *mockBeadClientForLabelTest) GetParent(b *bead.Bead) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForLabelTest) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForLabelTest) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForLabelTest) HasOpenChildren(parentID string) (bool, error) {
	return false, nil
}

type mockClaudeClient struct{}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	return &claude.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockClaudeClient) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	return &claude.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockClaudeClient) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	return &claude.Result{Success: true, Output: "mock output"}, nil
}

type mockAnalyzer struct{}

func (m *mockAnalyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
	return &analyzer.Analysis{}, nil
}

type mockRenderer struct{}

func (m *mockRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{}, nil
}

func (m *mockRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "mock prompt", nil
}

func (m *mockRenderer) LoadSpec(name string) (string, error) {
	return "mock spec", nil
}

func (m *mockRenderer) LoadClaudeMD() (string, error) {
	return "mock claude.md", nil
}

func (m *mockRenderer) LoadRules() (string, error) {
	return "mock rules", nil
}

func (m *mockRenderer) GetLearningsFile() *learnings.File {
	return nil
}

type mockLogger struct{}

func (m *mockLogger) LogIteration(log *logger.IterationLog) error {
	return nil
}

func (m *mockLogger) LogReview(log *logger.ReviewLog) error {
	return nil
}

func (m *mockLogger) Close() error {
	return nil
}

func (m *mockLogger) FilePath() string {
	return "/tmp/mock.log"
}
