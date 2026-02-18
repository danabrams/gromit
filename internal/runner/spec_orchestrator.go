package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
)

// newSpecOrchestrator creates a SpecOrchestrator wired to the runner's dependencies.
func newSpecOrchestrator(r *Runner) *SpecOrchestrator {
	return &SpecOrchestrator{
		renderer:    r.renderer,
		router:      r.router,
		beads:       r.beads,
		cfg:         r.cfg,
		cmdRunnerFn: r.cmdRunnerFn,
	}
}

// SpecOrchestrator coordinates spec-level acceptance test authoring.
type SpecOrchestrator struct {
	renderer    PromptRenderer
	router      *provider.Router
	beads       BeadClient
	cfg         *config.Config
	cmdRunnerFn func(ctx context.Context, command string, workDir string) (string, string, int, error)

	authoredSpecs map[string]bool
}

// AuthorAcceptanceTests loads the spec, renders the acceptance prompt, and invokes the provider.
func (o *SpecOrchestrator) AuthorAcceptanceTests(ctx context.Context, specName string) error {
	if o == nil {
		return fmt.Errorf("spec orchestrator is nil")
	}
	if o.renderer == nil {
		return fmt.Errorf("spec orchestrator renderer is nil")
	}
	if o.router == nil {
		return fmt.Errorf("spec orchestrator router is nil")
	}
	if specName == "" {
		return fmt.Errorf("spec name is empty")
	}

	if o.authoredSpecs != nil && o.authoredSpecs[specName] {
		return nil
	}

	spec, err := o.renderer.LoadSpec(specName)
	if err != nil {
		return fmt.Errorf("loading spec %s: %w", specName, err)
	}
	if strings.TrimSpace(spec) == "" {
		return fmt.Errorf("spec file .gromit/specs/%s.md not found", specName)
	}

	rules, err := o.renderer.LoadRulesForPhase("build")
	if err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}

	acceptancePrompt, err := o.renderer.RenderSpecAcceptance(&prompt.SpecAcceptanceContext{
		Spec:  spec,
		Rules: rules,
	})
	if err != nil {
		return fmt.Errorf("rendering spec acceptance prompt: %w", err)
	}

	tier := escalation.SelectTier(o.cfg, nil)

	p, _ := o.router.Select("build", tier)
	if p == nil {
		return fmt.Errorf("no provider available for spec acceptance")
	}

	result, err := p.Run(ctx, acceptancePrompt, tier)
	if err != nil {
		return fmt.Errorf("spec acceptance invocation: %w", err)
	}
	if result == nil {
		return fmt.Errorf("spec acceptance returned nil result")
	}
	if !result.Success {
		return fmt.Errorf("spec acceptance failed with exit code %d", result.ExitCode)
	}

	if err := o.commitAcceptanceTests(ctx, specName); err != nil {
		return err
	}

	if o.authoredSpecs == nil {
		o.authoredSpecs = make(map[string]bool)
	}
	o.authoredSpecs[specName] = true
	return nil
}

func (o *SpecOrchestrator) commitAcceptanceTests(ctx context.Context, specName string) error {
	_, stderr, exitCode, err := o.runCmd(ctx, "git add -A", "")
	if err != nil {
		return fmt.Errorf("staging acceptance tests: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("staging acceptance tests (exit %d): %s", exitCode, stderr)
	}

	message := fmt.Sprintf("test(spec): add acceptance tests for %s", specName)
	_, stderr, exitCode, err = o.runCmd(ctx, fmt.Sprintf("git commit -m %q", message), "")
	if err != nil {
		return fmt.Errorf("committing acceptance tests: %w", err)
	}
	if exitCode != 0 {
		if strings.Contains(strings.ToLower(stderr), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("committing acceptance tests (exit %d): %s", exitCode, stderr)
	}

	return nil
}

func (o *SpecOrchestrator) runCmd(ctx context.Context, command string, workDir string) (string, string, int, error) {
	if o.cmdRunnerFn != nil {
		return o.cmdRunnerFn(ctx, command, workDir)
	}
	return defaultCmdRunner(ctx, command, workDir)
}
