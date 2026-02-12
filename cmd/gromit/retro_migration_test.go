package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
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

	// Setup minimal config
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
		Paths: config.PathsConfig{
			GromitDir: tmpDir,
		},
	}

	// After migration, runRetro should:
	// 1. Create a Provider from config
	// 2. Call retro.NewRetroWithProvider(provider, gromitDir)
	//
	// Expected failure: runRetro still calls retro.NewRetro(cfg, gromitDir) at line 233

	// We can't directly test runRetro without complex mocking, but we can verify
	// that NewRetro no longer exists and NewRetroWithProvider is called correctly

	// Verify NewRetro does not exist (should fail to compile after migration)
	_, err := retro.NewRetro(cfg, tmpDir)
	if err == nil {
		t.Error("retro.NewRetro should not exist after migration - expected compile error")
	}

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
// Expected failure: runRetro doesn't create a Provider yet - still passes config directly
func TestRunRetro_ConstructsProviderFromConfig(t *testing.T) {
	// After migration, runRetro should create a Provider like:
	//   provider, err := providerFromConfig(cfg)
	// or
	//   claudeProvider := provider.NewClaudeProvider(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	//
	// Then pass it to:
	//   r, err := retro.NewRetroWithProvider(provider, gromitDir)
	//
	// Expected failure: providerFromConfig helper doesn't exist yet
	// Expected failure: runRetro still uses retro.NewRetro(cfg, gromitDir) without creating a provider

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
			Flags:   []string{"--no-input"},
		},
		Paths: config.PathsConfig{
			GromitDir: tmpDir,
		},
	}

	// The migration should add a helper function to construct a Provider from config
	// This could be a new function in cmd/gromit or internal/provider
	//
	// Expected failure: helper function doesn't exist yet
	p, err := providerFromConfig(cfg)
	if err != nil {
		t.Fatalf("providerFromConfig should exist after migration: %v", err)
	}

	if p == nil {
		t.Error("providerFromConfig should return non-nil provider")
	}

	// Verify the provider has the expected name
	if p.Name() != "claude" {
		t.Errorf("expected provider name 'claude', got %q", p.Name())
	}
}

// TestRetro_AcceptsOnlyProvider verifies that Retro constructor only accepts Provider,
// not config, after migration
// Expected failure: Retro still has NewRetro(cfg, gromitDir) constructor
func TestRetro_AcceptsOnlyProvider(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	// After migration, this should not compile - NewRetro should not exist
	// Expected failure: NewRetro still exists
	_, err := retro.NewRetro(cfg, tmpDir)
	if err == nil {
		t.Error("NewRetro(cfg, gromitDir) should not exist - migration should remove it")
	}

	// Only NewRetroWithProvider should exist
	mockProvider := &mockProviderForRetroCmd{
		runResult: &provider.Result{
			Success: true,
			Output:  "test",
		},
	}

	r, err := retro.NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Errorf("NewRetroWithProvider should exist after migration: %v", err)
	}
	if r == nil {
		t.Error("NewRetroWithProvider should return non-nil Retro")
	}
}

// TestProviderFromConfig_CreatesClaudeProvider verifies that the helper creates
// a ClaudeProvider from config
// Expected failure: providerFromConfig helper doesn't exist yet
func TestProviderFromConfig_CreatesClaudeProvider(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 120,
			Flags:   []string{"--no-input", "--debug"},
		},
	}

	// Expected failure: providerFromConfig doesn't exist
	p, err := providerFromConfig(cfg)
	if err != nil {
		t.Fatalf("providerFromConfig failed: %v", err)
	}

	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	// Verify it's a Claude provider
	if p.Name() != "claude" {
		t.Errorf("expected provider name 'claude', got %q", p.Name())
	}

	// Verify tier mapping works
	model := p.ModelForTier("high")
	if model == "" {
		t.Error("expected non-empty model for high tier")
	}
}

// TestProviderFromConfig_ErrorsOnNilConfig verifies error handling
// Expected failure: providerFromConfig doesn't exist yet
func TestProviderFromConfig_ErrorsOnNilConfig(t *testing.T) {
	// Expected failure: providerFromConfig doesn't exist
	p, err := providerFromConfig(nil)

	if err == nil {
		t.Error("expected error for nil config, got nil")
	}

	if p != nil {
		t.Error("expected nil provider for nil config")
	}
}

// TestRunRetro_Integration_UsesProvider verifies end-to-end that runRetro uses Provider
// Expected failure: runRetro still uses old NewRetro constructor
func TestRunRetro_Integration_UsesProvider(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal files for retro to work
	createMinimalRetroFilesForCmd(t, tmpDir)

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
			Flags:   []string{},
		},
		Paths: config.PathsConfig{
			GromitDir: tmpDir,
		},
	}

	// After migration, this flow should work:
	// 1. Create provider from config
	p, err := providerFromConfig(cfg)
	if err != nil {
		t.Fatalf("providerFromConfig failed: %v", err)
	}

	// 2. Create retro with provider
	r, err := retro.NewRetroWithProvider(p, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	if r == nil {
		t.Fatal("expected non-nil Retro")
	}

	// The old flow (should not compile after migration):
	// Expected failure: this still compiles because NewRetro exists
	_, legacyErr := retro.NewRetro(cfg, tmpDir)
	if legacyErr == nil {
		t.Error("legacy NewRetro should not exist after migration")
	}
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

func (m *mockProviderForRetroCmd) StreamRun(ctx context.Context, prompt string, tier string, output interface{}, handler interface{}, onToolCall interface{}) (*provider.Result, error) {
	return m.Run(ctx, prompt, tier)
}

func (m *mockProviderForRetroCmd) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.Run(ctx, "", tier)
}

func (m *mockProviderForRetroCmd) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
