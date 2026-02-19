package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

func TestSpecOrchestrator_AuthorAcceptanceTests_MissingSpecReturnsError(t *testing.T) {
	renderer := &mockPromptRenderer{
		LoadSpecFn: func(name string) (string, error) {
			return "", nil
		},
	}

	router := provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{})

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	orchestrator := &SpecOrchestrator{
		cfg:      cfg,
		router:   router,
		beads:    &mockBeadClient{},
		renderer: renderer,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "", 0, nil
		},
	}

	err := orchestrator.AuthorAcceptanceTests(context.Background(), "missing-spec")
	if err == nil {
		t.Fatal("expected error for missing spec, got nil")
	}
	if !strings.Contains(err.Error(), ".gromit/specs/missing-spec.md") {
		t.Fatalf("error should mention spec path, got %q", err.Error())
	}
}

func TestSpecOrchestrator_AuthorAcceptanceTests_LoadsSpecAndInvokesProvider(t *testing.T) {
	specContent := "# Spec"
	rulesContent := "Rules"
	var receivedSpec string
	var receivedRules string

	renderer := &mockPromptRenderer{
		LoadSpecFn: func(name string) (string, error) {
			if name != "demo-spec" {
				t.Fatalf("expected spec name demo-spec, got %q", name)
			}
			return specContent, nil
		},
		LoadRulesForPhaseFn: func(phase string) (string, error) {
			if phase != "build" {
				t.Fatalf("expected build phase, got %q", phase)
			}
			return rulesContent, nil
		},
		RenderSpecAcceptanceFn: func(ctx *prompt.SpecAcceptanceContext) (string, error) {
			receivedSpec = ctx.Spec
			receivedRules = ctx.Rules
			return "rendered prompt", nil
		},
	}

	var receivedPrompt string
	var receivedTier string
	providerRunCalls := 0
	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerRunCalls++
			receivedPrompt = prompt
			receivedTier = tier
			return &provider.Result{Success: true}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mockProvider)

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var commands []string
	orchestrator := &SpecOrchestrator{
		cfg:      cfg,
		router:   router,
		beads:    &mockBeadClient{},
		renderer: renderer,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			commands = append(commands, command)
			return "", "", 0, nil
		},
	}

	if err := orchestrator.AuthorAcceptanceTests(context.Background(), "demo-spec"); err != nil {
		t.Fatalf("AuthorAcceptanceTests returned error: %v", err)
	}
	if providerRunCalls != 1 {
		t.Fatalf("expected provider to be invoked once, got %d", providerRunCalls)
	}
	if receivedPrompt != "rendered prompt" {
		t.Fatalf("expected prompt to be rendered, got %q", receivedPrompt)
	}
	if receivedTier == "" {
		t.Fatal("expected tier to be set for provider run")
	}
	if receivedSpec != specContent {
		t.Fatalf("expected spec content %q, got %q", specContent, receivedSpec)
	}
	if receivedRules != rulesContent {
		t.Fatalf("expected rules %q, got %q", rulesContent, receivedRules)
	}
	if len(commands) == 0 {
		t.Fatal("expected git commands to run for committing tests")
	}
}

func TestSpecOrchestrator_AuthorAcceptanceTests_IdempotentBySpecName(t *testing.T) {
	renderer := &mockPromptRenderer{
		LoadSpecFn: func(name string) (string, error) {
			return "spec content", nil
		},
		LoadRulesForPhaseFn: func(phase string) (string, error) {
			return "rules", nil
		},
		RenderSpecAcceptanceFn: func(ctx *prompt.SpecAcceptanceContext) (string, error) {
			return "rendered prompt", nil
		},
	}

	providerRunCalls := 0
	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerRunCalls++
			return &provider.Result{Success: true}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mockProvider)

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	cmdCalls := 0
	orchestrator := &SpecOrchestrator{
		cfg:      cfg,
		router:   router,
		beads:    &mockBeadClient{},
		renderer: renderer,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			cmdCalls++
			return "", "", 0, nil
		},
	}

	if err := orchestrator.AuthorAcceptanceTests(context.Background(), "demo-spec"); err != nil {
		t.Fatalf("first AuthorAcceptanceTests returned error: %v", err)
	}
	if err := orchestrator.AuthorAcceptanceTests(context.Background(), "demo-spec"); err != nil {
		t.Fatalf("second AuthorAcceptanceTests returned error: %v", err)
	}

	if providerRunCalls != 1 {
		t.Fatalf("expected provider to be invoked once, got %d", providerRunCalls)
	}
	if cmdCalls == 0 {
		t.Fatal("expected git commands to run on first call")
	}
}

func TestSpecOrchestrator_CommitAcceptanceTests_StagesOnlyAcceptanceTests(t *testing.T) {
	var commands []string
	orchestrator := &SpecOrchestrator{
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			commands = append(commands, command)
			return "", "", 0, nil
		},
	}

	if err := orchestrator.commitAcceptanceTests(context.Background(), "demo-spec"); err != nil {
		t.Fatalf("commitAcceptanceTests returned error: %v", err)
	}
	if len(commands) == 0 {
		t.Fatal("expected git commands to run")
	}
	if commands[0] != "git add -- ':(glob)**/*_acceptance_test.go'" {
		t.Fatalf("expected scoped git add command, got %q", commands[0])
	}
}
