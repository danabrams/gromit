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
	var argvCalls []struct {
		program string
		args    []string
	}
	orchestrator := &SpecOrchestrator{
		cfg:      cfg,
		router:   router,
		beads:    &mockBeadClient{},
		renderer: renderer,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			commands = append(commands, command)
			return "", "", 0, nil
		},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			argvCalls = append(argvCalls, struct {
				program string
				args    []string
			}{program: program, args: append([]string(nil), args...)})
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
	if len(commands) != 0 {
		t.Fatalf("expected no shell command calls, got %v", commands)
	}
	if len(argvCalls) != 2 {
		t.Fatalf("expected two argv command calls, got %d", len(argvCalls))
	}
	if argvCalls[0].program != "git" || strings.Join(argvCalls[0].args, " ") != "add -- :(glob)**/*_acceptance_test.go" {
		t.Fatalf("unexpected first argv call: %#v", argvCalls[0])
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
	argvCalls := 0
	orchestrator := &SpecOrchestrator{
		cfg:      cfg,
		router:   router,
		beads:    &mockBeadClient{},
		renderer: renderer,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			cmdCalls++
			return "", "", 0, nil
		},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			argvCalls++
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
	if cmdCalls != 0 {
		t.Fatalf("expected no shell command calls, got %d", cmdCalls)
	}
	if argvCalls != 2 {
		t.Fatalf("expected two argv calls for first execution, got %d", argvCalls)
	}
}

func TestSpecOrchestrator_CommitAcceptanceTests_StagesOnlyAcceptanceTests(t *testing.T) {
	var programs []string
	var argvs [][]string
	orchestrator := &SpecOrchestrator{
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			programs = append(programs, program)
			argvs = append(argvs, append([]string(nil), args...))
			return "", "", 0, nil
		},
	}

	if err := orchestrator.commitAcceptanceTests(context.Background(), "demo-spec"); err != nil {
		t.Fatalf("commitAcceptanceTests returned error: %v", err)
	}
	if len(argvs) == 0 {
		t.Fatal("expected git commands to run")
	}
	if programs[0] != "git" {
		t.Fatalf("expected git program, got %q", programs[0])
	}
	if strings.Join(argvs[0], " ") != "add -- :(glob)**/*_acceptance_test.go" {
		t.Fatalf("expected scoped git add argv, got %#v", argvs[0])
	}
}
