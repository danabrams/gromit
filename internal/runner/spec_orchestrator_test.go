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

func TestSpecOrchestrator_RunArgv_UsesConfiguredArgvRunner(t *testing.T) {
	var gotProgram string
	var gotArgs []string
	var gotWorkDir string

	orchestrator := &SpecOrchestrator{
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			gotProgram = program
			gotArgs = append([]string(nil), args...)
			gotWorkDir = workDir
			return "ok", "warn", 7, nil
		},
	}

	stdout, stderr, exitCode, err := orchestrator.runArgv(context.Background(), "git", []string{"status", "--short"}, "/tmp")
	if err != nil {
		t.Fatalf("runArgv returned error: %v", err)
	}
	if stdout != "ok" || stderr != "warn" || exitCode != 7 {
		t.Fatalf("unexpected runArgv result: stdout=%q stderr=%q exit=%d", stdout, stderr, exitCode)
	}
	if gotProgram != "git" {
		t.Fatalf("expected program git, got %q", gotProgram)
	}
	if strings.Join(gotArgs, " ") != "status --short" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if gotWorkDir != "/tmp" {
		t.Fatalf("expected workDir /tmp, got %q", gotWorkDir)
	}
}

func TestSpecOrchestrator_RunArgv_FallsBackToDefaultArgvRunner(t *testing.T) {
	workDir := t.TempDir()
	orchestrator := &SpecOrchestrator{
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			t.Fatal("cmdRunnerFn should not be called by runArgv fallback")
			return "", "", 0, nil
		},
	}

	stdout, stderr, exitCode, err := orchestrator.runArgv(
		context.Background(),
		"sh",
		[]string{"-c", "printf '%s' \"$1\"", "ignored0", "argv-ok"},
		workDir,
	)
	if err != nil {
		t.Fatalf("runArgv returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", exitCode, stderr)
	}
	if stdout != "argv-ok" {
		t.Fatalf("expected stdout argv-ok, got %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	// Confirm non-interactive env is still applied by defaultArgvRunner.
	stdout, stderr, exitCode, err = orchestrator.runArgv(
		context.Background(),
		"sh",
		[]string{"-c", "printf '%s,%s,%s,%s' \"$GIT_TERMINAL_PROMPT\" \"$CI\" \"$NONINTERACTIVE\" \"$TERM\""},
		workDir,
	)
	if err != nil {
		t.Fatalf("env check runArgv returned error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected env check exit code 0, got %d (stderr=%q)", exitCode, stderr)
	}
	wantEnv := "0,1,1,dumb"
	if stdout != wantEnv {
		t.Fatalf("expected env %q, got %q", wantEnv, stdout)
	}
}

func TestSpecOrchestrator_RunArgv_FallbackReturnsNonZeroExitWithoutError(t *testing.T) {
	orchestrator := &SpecOrchestrator{}

	stdout, stderr, exitCode, err := orchestrator.runArgv(context.Background(), "sh", []string{"-c", "echo boom >&2; exit 13"}, "")
	if err != nil {
		t.Fatalf("runArgv returned unexpected error: %v", err)
	}
	if exitCode != 13 {
		t.Fatalf("expected exit code 13, got %d", exitCode)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "boom") {
		t.Fatalf("expected stderr to contain boom, got %q", stderr)
	}
}
