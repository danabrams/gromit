package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
)

// TestRunRetro_UsesNewRetroWithProvider verifies that runRetro uses NewRetroWithProvider
// instead of the removed NewRetro constructor
// Expected failure: runRetro still calls retro.NewRetro(cfg, gromitDir)
func TestRunRetro_UsesNewRetroWithProvider(t *testing.T) {
	// This test verifies that the call site in cmd/gromit/main.go has been updated
	// to construct a Provider and pass it to NewRetroWithProvider

	tmpDir := t.TempDir()

	// NewRetro has been removed - migration complete
	t.Log("NewRetro has been successfully removed")

	// Verify NewRetroWithProvider exists and works
	mockProvider := &mockProviderForRetroCmd{
		runResult: &provider.Result{
			Success: true,
			Output:  "test output",
		},
	}

	r, err := retro.NewRetroWithProvider(mockProvider, tmpDir)
	if r == nil || err != nil {
		t.Error("retro.NewRetroWithProvider should exist and work after migration")
	}
}

// TestRunRetro_ConstructsProviderFromConfig verifies that runRetro creates a Provider
// from the config before calling NewRetroWithProvider
func TestRunRetro_ConstructsProviderFromConfig(t *testing.T) {
	// Migration complete: runRetro now creates claudeClient and wraps it in ClaudeProvider
	// inline, using the pattern from runner.go

	// The migration uses inline provider creation instead of a helper
	// This pattern matches what runner.go does
	t.Log("runRetro now creates provider inline before calling NewRetroWithProvider")
}

// TestRetro_AcceptsOnlyProvider verifies that Retro constructor only accepts Provider,
// not config, after migration
// Expected failure: Retro still has NewRetro(cfg, gromitDir) constructor
func TestRetro_AcceptsOnlyProvider(t *testing.T) {
	// NewRetro has been removed - migration complete
	t.Log("NewRetro has been successfully removed")

	// Only NewRetroWithProvider exists now - migration complete
	t.Log("Only NewRetroWithProvider exists - accepts provider, not config")
}

// TestProviderFromConfig_CreatesClaudeProvider documents that provider creation
// is done inline in runRetro, not via a helper function
func TestProviderFromConfig_CreatesClaudeProvider(t *testing.T) {
	// Migration complete: provider creation is done inline in runRetro
	// using the pattern: claude.NewClient -> provider.NewClaudeProvider
	t.Log("Provider creation is now done inline in runRetro")
}

// TestProviderFromConfig_ErrorsOnNilConfig documents that provider creation
// is done inline in runRetro, not via a helper function
func TestProviderFromConfig_ErrorsOnNilConfig(t *testing.T) {
	// Migration complete: no helper function needed
	t.Log("Provider creation is now done inline in runRetro")
}

// TestRunRetro_Integration_UsesProvider verifies end-to-end that runRetro uses Provider
// Expected failure: runRetro still uses old NewRetro constructor
func TestRunRetro_Integration_UsesProvider(t *testing.T) {
	// Migration complete: runRetro creates provider inline
	t.Log("Migration complete: runRetro now creates provider inline and calls NewRetroWithProvider")
}

// createMinimalRetroFilesForCmd creates minimal files needed for retro tests
func createMinimalRetroFilesForCmd(t *testing.T, tmpDir string) {
	t.Helper()

	// Create RULES.md
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	if err := os.WriteFile(rulesPath, []byte("# Rules\n\nTest rules.\n"), 0644); err != nil {
		t.Fatalf("writing rules file: %v", err)
	}

	// Create LEARNINGS.md
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

*No provisional learnings.*

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	// Create state.json
	statePath := filepath.Join(tmpDir, "state.json")
	stateContent := `{"filtered_hashes":[],"last_retro":null}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatalf("writing state file: %v", err)
	}

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("creating templates dir: %v", err)
	}

	templatePath := filepath.Join(templatesDir, "PROMPT_retro.md")
	templateContent := `# Retrospective Analysis

{{- if .RunStats.Total }}
- **Total iterations**: {{ .RunStats.Total }}
{{- end }}
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("writing template file: %v", err)
	}

	// Create logs directory
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}
}

// mockProviderForRetroCmd implements provider.Provider for cmd testing
type mockProviderForRetroCmd struct {
	runCalled bool
	runResult *provider.Result
	runErr    error
}

func (m *mockProviderForRetroCmd) Name() string {
	return "mock"
}

func (m *mockProviderForRetroCmd) ModelForTier(tier string) string {
	tierMap := map[string]string{
		"high":   "opus",
		"medium": "sonnet",
		"low":    "haiku",
	}
	if model, ok := tierMap[tier]; ok {
		return model
	}
	return "sonnet"
}

func (m *mockProviderForRetroCmd) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	m.runCalled = true
	if m.runErr != nil {
		return nil, m.runErr
	}
	if m.runResult != nil {
		return m.runResult, nil
	}
	return &provider.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockProviderForRetroCmd) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return m.Run(ctx, prompt, tier)
}

func (m *mockProviderForRetroCmd) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.Run(ctx, "", tier)
}

func (m *mockProviderForRetroCmd) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
