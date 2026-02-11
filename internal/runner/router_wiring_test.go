package runner

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// Expected failure: Router field does not exist on Runner struct yet
func TestRunnerHasRouterField(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       tmpDir + "/templates",
			Specs:           tmpDir + "/specs",
			Logs:            tmpDir + "/logs",
			ProjectClaudeMD: tmpDir + "/CLAUDE.md",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create necessary directories and files
	_ = os.MkdirAll(cfg.Paths.Templates, 0755)
	_ = os.WriteFile(cfg.Paths.ProjectClaudeMD, []byte("# Test"), 0644)

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	// Expected failure: runner.router field does not exist yet
	if runner.router == nil {
		t.Error("Runner.router is nil, expected non-nil router instance")
	}
}

// Expected failure: Router field does not exist on Deps struct yet
func TestDepsStructHasRouterField(t *testing.T) {
	deps := Deps{
		Beads:    &mockBeadClient{},
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	// Expected failure: deps.Router field does not exist yet
	if deps.Router != nil {
		t.Error("Deps.Router should start as nil")
	}

	// Set a mock router
	mockRouter := &mockRouter{}
	deps.Router = mockRouter

	// Expected failure: deps.Router field does not exist yet
	if deps.Router != mockRouter {
		t.Error("Deps.Router not assigned correctly")
	}
}

// Expected failure: NewRunner does not build providers or create Router yet
func TestNewRunnerCreatesRouterWhenProvidersConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       tmpDir + "/templates",
			Specs:           tmpDir + "/specs",
			Logs:            tmpDir + "/logs",
			ProjectClaudeMD: tmpDir + "/CLAUDE.md",
		},
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary:         "claude",
				Flags:          []string{"--no-input"},
				PromptDelivery: "stdin",
				Models: map[string]string{
					"high":   "opus",
					"medium": "sonnet",
					"low":    "haiku",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build": "claude",
			},
			Ratio: map[string]int{
				"claude": 100,
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create necessary directories and files
	_ = os.MkdirAll(cfg.Paths.Templates, 0755)
	_ = os.WriteFile(cfg.Paths.ProjectClaudeMD, []byte("# Test"), 0644)

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	// Expected failure: runner.router does not exist yet, and NewRunner does not build providers
	if runner.router == nil {
		t.Error("Runner.router is nil when providers are configured, expected Router to be built from providers config")
	}

	// Verify the router was created with the configured providers
	// Expected failure: Select method does not exist on router yet
	selectedProvider, modelName := runner.router.Select("build", "high")
	if selectedProvider == nil {
		t.Error("Router.Select returned nil provider for build phase with high tier")
	}
	if modelName == "" {
		t.Error("Router.Select returned empty model name")
	}
}

// Expected failure: NewRunner does not wrap Claude in NewSingleProviderRouter yet
func TestNewRunnerUsesClaudeWrapperWhenNoProviders(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       tmpDir + "/templates",
			Specs:           tmpDir + "/specs",
			Logs:            tmpDir + "/logs",
			ProjectClaudeMD: tmpDir + "/CLAUDE.md",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create necessary directories and files
	_ = os.MkdirAll(cfg.Paths.Templates, 0755)
	_ = os.WriteFile(cfg.Paths.ProjectClaudeMD, []byte("# Test"), 0644)

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	// Expected failure: runner.router does not exist yet, and NewRunner does not create single-provider router
	if runner.router == nil {
		t.Error("Runner.router is nil when no providers configured, expected single-provider router wrapping ClaudeClient")
	}

	// Verify the router contains a single provider
	// Expected failure: Select method does not exist on router yet
	selectedProvider, modelName := runner.router.Select("build", "high")
	if selectedProvider == nil {
		t.Error("Router.Select returned nil provider when using single-provider fallback")
	}
	if modelName == "" {
		t.Error("Router.Select returned empty model name from single-provider router")
	}
}

// Expected failure: NewRunnerWithDeps does not use deps.Router yet
func TestNewRunnerWithDepsUsesProvidedRouter(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       tmpDir + "/templates",
			Specs:           tmpDir + "/specs",
			Logs:            tmpDir + "/logs",
			ProjectClaudeMD: tmpDir + "/CLAUDE.md",
		},
	}

	// Create necessary directories and files
	_ = os.MkdirAll(cfg.Paths.Templates, 0755)
	_ = os.WriteFile(cfg.Paths.ProjectClaudeMD, []byte("# Test"), 0644)

	mockRouterInstance := &mockRouter{
		selectResult: &mockProvider{name: "test-provider"},
		modelName:    "test-model",
	}

	deps := Deps{
		Beads:    &mockBeadClient{},
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
		Router:   mockRouterInstance,
	}

	runner, err := NewRunnerWithDeps(cfg, os.Stdout, tmpDir, deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Expected failure: runner.router does not exist yet, and NewRunnerWithDeps does not use deps.Router
	if runner.router != mockRouterInstance {
		t.Error("NewRunnerWithDeps did not use deps.Router, expected runner.router to match provided Router")
	}

	// Verify the router is the same instance
	// Expected failure: Select method does not exist on runner.router yet
	selectedProvider, _ := runner.router.Select("build", "high")
	if mockProv, ok := selectedProvider.(*mockProvider); !ok || mockProv.name != "test-provider" {
		t.Error("Router returned by runner is not the same instance provided in deps")
	}
}

// Expected failure: NewRunnerWithDeps does not create fallback router when deps.Router is nil
func TestNewRunnerWithDepsFallsBackToClaudeWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       tmpDir + "/templates",
			Specs:           tmpDir + "/specs",
			Logs:            tmpDir + "/logs",
			ProjectClaudeMD: tmpDir + "/CLAUDE.md",
		},
	}

	// Create necessary directories and files
	_ = os.MkdirAll(cfg.Paths.Templates, 0755)
	_ = os.WriteFile(cfg.Paths.ProjectClaudeMD, []byte("# Test"), 0644)

	mockClaude := &mockClaudeClient{}
	deps := Deps{
		Beads:    &mockBeadClient{},
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
		Router:   nil, // No router provided
	}

	runner, err := NewRunnerWithDeps(cfg, os.Stdout, tmpDir, deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Expected failure: runner.router does not exist yet, and NewRunnerWithDeps does not wrap Claude
	if runner.router == nil {
		t.Error("NewRunnerWithDeps did not create fallback router when deps.Router is nil")
	}

	// Verify the router wraps the Claude client for backward compatibility
	// Expected failure: Select method does not exist on runner.router yet
	selectedProvider, _ := runner.router.Select("build", "high")
	if selectedProvider == nil {
		t.Error("Router from Claude wrapper returned nil provider")
	}
}

// Mock implementations for testing

type mockRouter struct {
	selectResult provider.Provider
	modelName    string
	selectCalls  []mockSelectCall
}

type mockSelectCall struct {
	phase string
	tier  string
}

func (m *mockRouter) Select(phase string, tier string) (provider.Provider, string) {
	m.selectCalls = append(m.selectCalls, mockSelectCall{phase: phase, tier: tier})
	if m.selectResult != nil {
		return m.selectResult, m.modelName
	}
	return nil, ""
}

func (m *mockRouter) MarkUnavailable(name string) {}

func (m *mockRouter) RecordInvocation(name string) {}

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{
		Success:  true,
		Output:   "mock output",
		ExitCode: 0,
		Model:    m.name + "-" + tier,
	}, nil
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{
		Success:  true,
		Output:   "mock output",
		ExitCode: 0,
		Model:    m.name + "-" + tier,
	}, nil
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{
		Success:  true,
		Output:   "VALIDATION_PASSED",
		ExitCode: 0,
		Model:    m.name + "-" + tier,
	}, nil
}

func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
