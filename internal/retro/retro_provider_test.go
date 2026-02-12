package retro

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestNewRetro_AcceptsProvider verifies that NewRetro can accept a Provider for learnings filtering and analysis
func TestNewRetro_AcceptsProvider(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock provider
	mockProvider := &mockProvider{}

	// NewRetro should accept a provider parameter
	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	if r == nil {
		t.Fatal("expected non-nil Retro")
	}

	if r.provider != mockProvider {
		t.Error("expected Retro to store the provider")
	}
}

// TestRetro_Run_UsesProviderForLearningsFilter verifies that Run uses provider for learnings filtering
func TestRetro_Run_UsesProviderForLearningsFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal files for retro to run
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProvider{
		runResult: &provider.Result{
			Success: true,
			Output:  `{"analysis": "test analysis"}`,
		},
	}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// Run should use the provider
	_, err = r.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify provider was called
	if !mockProvider.runCalled {
		t.Error("expected provider.Run to be called")
	}
}

// createMinimalRetroFilesForProvider creates minimal files needed for retro.Run()
func createMinimalRetroFilesForProvider(t *testing.T, tmpDir string) {
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

	// Create templates directory with retro template
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

	// Create logs directory (empty is fine)
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}
}

// mockProvider implements provider.Provider for testing
type mockProvider struct {
	runCalled bool
	runResult *provider.Result
	runErr    error
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	m.runCalled = true
	if m.runErr != nil {
		return nil, m.runErr
	}
	if m.runResult != nil {
		return m.runResult, nil
	}
	return &provider.Result{Success: true, Output: `{"category":"logic","recoverable":true,"root_cause":"test","suggestion":"test"}`}, nil
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output interface{}, handler interface{}, onToolCall interface{}) (*provider.Result, error) {
	return m.Run(ctx, prompt, tier)
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.Run(ctx, "", tier)
}

func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
