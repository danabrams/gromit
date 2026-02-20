package runner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
)

const (
	buildPhase                = "build"
	specAcceptanceTestsHeader = "## Existing Spec-Level Tests"
	gitAcceptanceTestsGlob    = ":(glob)**/*_acceptance_test.go"
)

// newSpecOrchestrator creates a SpecOrchestrator wired to the runner's dependencies.
func newSpecOrchestrator(r *Runner) *SpecOrchestrator {
	var (
		renderer     PromptRenderer
		router       *provider.Router
		beads        BeadClient
		cfg          *config.Config
		argvRunnerFn ArgvRunnerFn
		logFn        func(format string, args ...interface{})
		specsDir     string
	)
	if r != nil {
		renderer = r.renderer
		router = r.router
		beads = r.beads
		cfg = r.cfg
		argvRunnerFn = r.argvRunnerFn
		logFn = r.log
		if r.cfg != nil {
			specsDir = r.cfg.Paths.Specs
		}
	}

	return &SpecOrchestrator{
		renderer:          renderer,
		router:            router,
		beads:             beads,
		specContentLoader: &fileSpecContentLoader{specsDir: specsDir},
		cfg:               cfg,
		argvRunnerFn:      argvRunnerFn,
		logFn:             logFn,
	}
}

// SpecContentLoader loads spec content and existing spec-level tests.
type SpecContentLoader interface {
	LoadSpecContent(specName string) (string, error)
	LoadExistingSpecTests(specName string) (string, error)
}

// fileSpecContentLoader loads specs and existing tests from the local filesystem.
type fileSpecContentLoader struct {
	specsDir string
}

var _ SpecContentLoader = (*fileSpecContentLoader)(nil)

func (l *fileSpecContentLoader) LoadSpecContent(specName string) (string, error) {
	if l == nil {
		return "", fmt.Errorf("spec content loader is nil")
	}
	if err := prompt.ValidateSpecName(specName); err != nil {
		return "", err
	}
	path := filepath.Join(l.specsDir, specName+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading spec %s: %w", specName, err)
	}
	return string(content), nil
}

func (l *fileSpecContentLoader) LoadExistingSpecTests(specName string) (string, error) {
	if l == nil {
		return "", fmt.Errorf("spec content loader is nil")
	}
	if err := prompt.ValidateSpecName(specName); err != nil {
		return "", err
	}

	matches := make([]string, 0)
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_acceptance_test.go") {
			return nil
		}
		normalized := filepath.ToSlash(path)
		if strings.Contains(normalized, specName) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking existing acceptance tests: %w", err)
	}
	if len(matches) == 0 {
		return "", nil
	}

	slices.Sort(matches)
	var sections []string
	for _, match := range matches {
		content, readErr := os.ReadFile(match)
		if readErr != nil {
			return "", fmt.Errorf("reading acceptance test %s: %w", match, readErr)
		}
		sections = append(sections, fmt.Sprintf("### %s\n%s", filepath.ToSlash(match), string(content)))
	}
	return strings.Join(sections, "\n\n"), nil
}

// SpecOrchestrator coordinates spec-level acceptance test authoring.
type SpecOrchestrator struct {
	renderer          PromptRenderer
	router            *provider.Router
	beads             BeadClient
	specContentLoader SpecContentLoader
	cfg               *config.Config
	argvRunnerFn      ArgvRunnerFn
	logFn             func(format string, args ...interface{})

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

	spec, err := o.loadSpecContent(specName)
	if err != nil {
		return fmt.Errorf("loading spec %s: %w", specName, err)
	}
	if strings.TrimSpace(spec) == "" {
		o.logWarning("skipping spec acceptance authoring for %q: spec file %s not found", specName, filepath.ToSlash(filepath.Join(o.specsDirOrDefault(), specName+".md")))
		return nil
	}

	existingTests, err := o.loadExistingSpecTests(specName)
	if err != nil {
		return fmt.Errorf("loading existing acceptance tests for %s: %w", specName, err)
	}

	rules, err := o.renderer.LoadRulesForPhase(buildPhase)
	if err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}

	specForPrompt := spec
	if strings.TrimSpace(existingTests) != "" {
		specForPrompt = strings.TrimSpace(spec) + "\n\n" + specAcceptanceTestsHeader + "\n\n" + strings.TrimSpace(existingTests)
	}

	acceptancePrompt, err := o.renderer.RenderSpecAcceptance(&prompt.SpecAcceptanceContext{
		Spec:  specForPrompt,
		Rules: rules,
	})
	if err != nil {
		return fmt.Errorf("rendering spec acceptance prompt: %w", err)
	}

	tier := escalation.SelectTier(o.cfg, nil)

	p, modelName := o.router.Select(buildPhase, tier)
	if p == nil {
		return fmt.Errorf("no provider available for spec acceptance")
	}

	result, err := p.Run(ctx, acceptancePrompt, tier)
	if err != nil && p.IsUsageLimitError(result, err) {
		o.router.MarkUnavailable(p.Name())
		if fallback, fallbackModel := o.router.Select(buildPhase, tier); fallback != nil {
			p = fallback
			modelName = fallbackModel
			result, err = p.Run(ctx, acceptancePrompt, tier)
		}
	}
	if err != nil {
		return fmt.Errorf("spec acceptance invocation failed (provider=%s model=%s): %w", p.Name(), modelName, err)
	}
	if result == nil {
		return fmt.Errorf("spec acceptance failed (provider=%s model=%s): nil result", p.Name(), modelName)
	}
	if !result.Success {
		return fmt.Errorf(
			"spec acceptance failed (provider=%s model=%s): exit=%d failure_category=%s",
			p.Name(),
			modelName,
			result.ExitCode,
			result.FailureCategory,
		)
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
	_, stderr, exitCode, err := o.runArgv(ctx, "git", []string{"add", "--", gitAcceptanceTestsGlob}, "")
	if err != nil {
		return fmt.Errorf("staging acceptance tests: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("staging acceptance tests (exit %d): %s", exitCode, stderr)
	}

	message := fmt.Sprintf("test(spec): add acceptance tests for %s", specName)
	_, stderr, exitCode, err = o.runArgv(ctx, "git", []string{"commit", "-m", message}, "")
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

func (o *SpecOrchestrator) runArgv(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
	if o.argvRunnerFn != nil {
		return o.argvRunnerFn(ctx, program, args, workDir)
	}
	return defaultArgvRunner(ctx, program, args, workDir)
}

func (o *SpecOrchestrator) loadSpecContent(specName string) (string, error) {
	if o != nil && o.specContentLoader != nil {
		return o.specContentLoader.LoadSpecContent(specName)
	}
	if o != nil && o.renderer != nil {
		return o.renderer.LoadSpec(specName)
	}
	return "", fmt.Errorf("spec content loader is nil")
}

func (o *SpecOrchestrator) loadExistingSpecTests(specName string) (string, error) {
	if o != nil && o.specContentLoader != nil {
		return o.specContentLoader.LoadExistingSpecTests(specName)
	}
	return "", nil
}

func (o *SpecOrchestrator) specsDirOrDefault() string {
	if o != nil && o.cfg != nil && strings.TrimSpace(o.cfg.Paths.Specs) != "" {
		return o.cfg.Paths.Specs
	}
	return ".gromit/specs"
}

func (o *SpecOrchestrator) logWarning(format string, args ...interface{}) {
	if o == nil || o.logFn == nil {
		return
	}
	o.logFn("Warning: "+format, args...)
}
