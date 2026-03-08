package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// PromptFragments groups template fragments for each build methodology.
type PromptFragments struct {
	Standard string
	TDD      string
	Refactor string
}

// Methodology represents a build methodology.
type Methodology string

const (
	MethodologyStandard Methodology = "standard"
	MethodologyTDD      Methodology = "tdd"
	MethodologyRefactor Methodology = "refactor"
)

const defaultBuildFragment = `# Build Instructions — Standard Methodology

You are executing a single task from the work queue. Focus only on this task.

## Scope Boundary

Your scope is EXACTLY the bead described in the instance context above.

- Implement ONLY what the bead title describes
- Do NOT add features beyond the task scope
- Note follow-on work in commit messages, do not do it

## Instructions

1. Study the codebase before making changes
2. Implement the task following existing patterns
3. Write tests if the task involves new functionality
4. Self-check — run go test and go vet scoped to touched packages. Fix failures before committing
5. Commit your changes with a clear commit message

Do NOT ask questions or request confirmation — execute the task directly.
`

const defaultBuildTDDFragment = `# Build Instructions — TDD Methodology

You are executing a single task using Test-Driven Development (TDD). Focus only on this task.

## Scope Boundary

Your scope is EXACTLY the bead described in the instance context above.

- Implement ONLY what the bead title describes — nothing upstream or downstream
- Do NOT implement consumers, CLI flags, or wiring for the thing you're adding
- Do NOT add features that "would be nice to have" alongside the task

## Instructions — Red-Green-Refactor Discipline

You MUST follow red-green-refactor strictly. Each cycle is small and committed separately.

### The Cycle (repeat for each requirement)

**1. RED — Write ONE failing test**
- Write a single test function or test case that calls code which doesn't exist yet or doesn't behave correctly yet
- Run tests: they MUST fail (compilation errors count as failing)
- Commit: ` + "`red: test for <what the test verifies>`" + `
- Do NOT write any production code in this step

**2. GREEN — Write minimum production code**
- Write only enough production code to make the failing test pass
- Do NOT modify the test you just wrote
- Do NOT add anything beyond what this one test requires
- Run tests: they MUST pass
- Commit: ` + "`green: implement <what you added>`" + `

**3. COMMIT and move to next requirement** — refactoring happens in a separate phase

### Non-Negotiable Rules

- ONE test per red step. Stop after writing it.
- MINIMUM code per green step. No "while I'm here" additions.
- SEPARATE commits for red and green. Each commit message starts with ` + "`red:`" + ` or ` + "`green:`" + `.
- Do NOT batch multiple requirements into one cycle.
- Before completing, run ` + "`go test`" + ` and ` + "`go vet`" + ` scoped to touched packages (not ` + "`./...`" + `). Fix any failures before committing.
- After all requirements are covered, stop — refactoring happens in a separate phase.

## Completion

When complete:
- Multiple small commits exist, alternating ` + "`red:`" + ` and ` + "`green:`" + ` prefixes
- All tests pass
- Each requirement has a corresponding test
- No gold plating — minimum viable implementation only

Do NOT output any special completion markers — just complete the task and exit.
Do NOT ask questions or request confirmation — execute the task directly.
`

const defaultBuildRefactorFragment = `# Build Instructions — Refactor Methodology

You are refactoring the implementation after tests pass. Your goal is to improve code quality without changing behavior.

## CRITICAL CONSTRAINT

You are in the **REFACTOR phase** of TDD. All tests are passing. You may improve code structure but you MUST NOT change behavior.

**After you finish, tests will run automatically. Tests MUST still PASS.** If any test fails, your refactoring is reverted.

**What you CAN do:**
- Rename variables, functions, or types for clarity
- Extract helpers or reduce duplication
- Simplify control flow or error handling
- Add constants for magic values
- Reorganize code within files

**What you MUST NOT do:**
- Add new features or new test cases — that's the next red phase
- Change what any function returns for a given input
- Delete or skip tests
- Make large rewrites — keep changes small and safe

## Instructions

1. **Review** the implementation for readability, duplication, naming, and adherence to project patterns
2. **Refactor** only what genuinely improves clarity — if the code is already clean, make no changes and say so
3. **Verify** by running scoped tests: ` + "`go test`" + ` and ` + "`go vet`" + ` on touched packages only (not ` + "`./...`" + `)
4. **Commit** with message: ` + "`refactor: <what you improved>`" + `

## Important Notes

- Only refactor code touched by this task, not the entire codebase
- Follow the project's existing patterns — don't introduce new conventions
- Small, safe improvements only — not wholesale rewrites

## Completion

When complete:
- Code quality improvements are committed (if any were needed)
- All tests still pass (behavior unchanged)
- Changes follow project conventions

Do NOT output any special completion markers — just complete the task and exit.
Do NOT ask questions or request confirmation — execute the task directly.
`

// BuildArtifacts exposes telemetry and output returned by the build stage.
type BuildArtifacts struct {
	Model        string
	Prompt       string
	Output       string
	Tokens       int // total (input + output), kept for backward compat
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Duration     time.Duration
	Success      bool
}

// Stage implements the build stage.
type Stage struct {
	name      string
	cfg       *config.Config
	llm       llmtypes.LLMProvider
	base      string
	project   string
	fragments PromptFragments
	output    io.Writer
	events.EmitterMixin
}

// New constructs a build stage backed by the provided dependencies.
func New(cfg *config.Config, provider llmtypes.LLMProvider, base, project string, fragments PromptFragments, output io.Writer) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if output == nil {
		output = io.Discard
	}
	return &Stage{
		name:      stagedesc.Describe("build", cfg),
		cfg:       cfg,
		llm:       provider,
		base:      base,
		project:   project,
		fragments: fragments,
		output:    output,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical build stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// RetryConfig returns the retry configuration for the build stage.
func (s *Stage) RetryConfig() stagepkg.RetryConfig {
	if s == nil || s.cfg == nil {
		return stagepkg.RetryConfig{}
	}
	maxRetries := s.cfg.Escalation.MaxRetriesPerModel
	if maxRetries <= 0 {
		maxRetries = 1
	}
	return stagepkg.RetryConfig{MaxRetries: maxRetries}
}

// Run executes the build LLM invocation, handles model escalation, and reports results.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	cfg, err := s.resolveConfig(req)
	if err != nil {
		return nil, err
	}

	methodology := s.resolveMethodology(req.Bead.Labels, cfg)
	fragment := s.fragmentFor(methodology)
	instance := buildInstanceLayer(req)

	// Load spec content if the bead has a spec label.
	if specName := bead.FindSpecLabel(req.Bead.Labels); specName != "" {
		specContent := loadSpecFile(req.Worktree, specName)
		if specContent != "" {
			if instance != "" {
				instance += "\n\n"
			}
			instance += "## Spec: " + specName + "\n\n" + specContent
		}
	}

	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, fragment).Assemble("build", prompt.BeadInfo{
		Title: req.Bead.Title,
	})

	model := s.selectModel(req, cfg)

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	resp, finalModel, invokeErr := s.invokeWithEscalation(ctx, provider, promptText, model, cfg, req.Worktree, req.Tier)
	if invokeErr != nil {
		return nil, fmt.Errorf("build: %w", invokeErr)
	}

	artifacts := &BuildArtifacts{
		Model:        finalModel,
		Prompt:       promptText,
		Output:       resp.Output,
		Tokens:       resp.Tokens,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		CostUSD:      resp.CostUSD,
		Duration:     resp.Duration,
		Success:      resp.Success,
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts, Events: []event.TypedEvent{}}, nil
}

func (s *Stage) resolveConfig(req *stagepkg.Request) (*config.Config, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return cfg, nil
}

func (s *Stage) fragmentFor(methodology Methodology) string {
	switch methodology {
	case MethodologyTDD:
		if s.fragments.TDD != "" {
			return s.fragments.TDD
		}
		return defaultBuildTDDFragment
	case MethodologyRefactor:
		if s.fragments.Refactor != "" {
			return s.fragments.Refactor
		}
		return defaultBuildRefactorFragment
	default:
		if s.fragments.Standard != "" {
			return s.fragments.Standard
		}
		return defaultBuildFragment
	}
}

func (s *Stage) resolveMethodology(labels []string, cfg *config.Config) Methodology {
	methodology := selectMethodology(labels, cfg)
	strategy := resolveBuildStrategy(cfg, labels)

	if strings.EqualFold(strategy, "tdd") {
		return MethodologyTDD
	}
	if strings.EqualFold(strategy, "single_pass") && hasBuildStrategyLabel(labels, "build_strategy:single_pass") {
		if methodology == MethodologyTDD || methodology == MethodologyRefactor {
			return MethodologyStandard
		}
	}
	return methodology
}

func selectMethodology(labels []string, cfg *config.Config) Methodology {
	tddGlobal := cfg != nil && cfg.Methodology.TDD
	if bead.IsMethodologyActive(labels, "tdd", tddGlobal) {
		return MethodologyTDD
	}
	if bead.IsMethodologyActive(labels, "refactor", false) {
		return MethodologyRefactor
	}
	return MethodologyStandard
}

func resolveBuildStrategy(cfg *config.Config, labels []string) string {
	strategy := "single_pass"
	if cfg != nil && cfg.Methodology.BuildStrategy != "" {
		strategy = strings.TrimSpace(cfg.Methodology.BuildStrategy)
	}
	for _, label := range labels {
		switch label {
		case "build_strategy:tdd":
			strategy = "tdd"
		case "build_strategy:single_pass":
			strategy = "single_pass"
		}
	}
	return strategy
}

func hasBuildStrategyLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

func buildInstanceLayer(req *stagepkg.Request) string {
	if req == nil {
		return ""
	}
	var builder strings.Builder
	if title := strings.TrimSpace(req.Bead.Title); title != "" {
		builder.WriteString("Task: ")
		builder.WriteString(title)
	}
	if desc := strings.TrimSpace(req.Bead.Description); desc != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("Description: ")
		builder.WriteString(desc)
	}
	if id := strings.TrimSpace(req.Bead.ID); id != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("Bead ID: ")
		builder.WriteString(id)
	}
	if req.RetryContext != nil && len(req.RetryContext.PriorFailures) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("Prior failures:")
		for _, failure := range req.RetryContext.PriorFailures {
			builder.WriteString("\n- ")
			builder.WriteString(failure)
		}
	}
	return builder.String()
}

// loadSpecFile reads a spec file from .gromit/specs/{name}.md relative to the
// given directory. Returns empty string if the file cannot be read.
func loadSpecFile(dir, specName string) string {
	if dir == "" || specName == "" {
		return ""
	}
	path := filepath.Join(dir, ".gromit", "specs", specName+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *Stage) selectModel(req *stagepkg.Request, cfg *config.Config) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Model); trimmed != "" {
			return trimmed
		}
	}
	if cfg != nil {
		if trimmed := strings.TrimSpace(cfg.Models.P1); trimmed != "" {
			return trimmed
		}
		if len(cfg.Escalation.Chain) > 0 {
			return cfg.Escalation.Chain[0]
		}
	}
	return config.ModelSonnet
}

func (s *Stage) writer() io.Writer {
	if s.output != nil {
		return s.output
	}
	return io.Discard
}

func (s *Stage) invokeWithEscalation(ctx context.Context, provider llmtypes.LLMProvider, prompt, initialModel string, cfg *config.Config, dir, tier string) (*llmtypes.LLMInvokeResponse, string, error) {
	model := initialModel
	escalationCfg := cfg
	if escalationCfg == nil {
		escalationCfg = s.cfg
	}

	seen := map[string]bool{model: true}
	chainLen := 0
	if escalationCfg != nil {
		chainLen = len(escalationCfg.Escalation.Chain)
	}
	maxIter := chainLen + 1 // safety bound

	for i := 0; i < maxIter; i++ {
		resp, err := provider.StreamInvoke(ctx, llmtypes.LLMStreamInvokeRequest{Prompt: prompt, Model: model, Output: s.writer(), Dir: dir, Metadata: map[string]string{"tier": tier}})
		if err == nil && resp != nil && resp.Success {
			return resp, model, nil
		}

		var reason error
		if err != nil {
			reason = err
		} else if resp == nil {
			reason = fmt.Errorf("provider returned nil response")
		} else if !resp.Success {
			detail := strings.TrimSpace(resp.Output)
			if detail == "" {
				detail = "no detail available"
			}
			reason = fmt.Errorf("provider reported unsuccessful result: %s", detail)
		}

		if escalationCfg == nil || !escalationCfg.Escalation.Enabled {
			return resp, model, reason
		}

		nextModel := escalationCfg.NextEscalationModel(model)
		if nextModel == "" || seen[nextModel] {
			return resp, model, reason
		}
		seen[nextModel] = true
		model = nextModel
	}

	// Should not be reached, but return the last state as a safety net.
	return nil, model, fmt.Errorf("escalation exhausted after %d iterations", maxIter)
}

// WithEmitter attaches an emitter for downstream consumers.
func (s *Stage) WithEmitter(emitter *events.Emitter) *Stage {
	s.EmitterMixin.SetEmitter(emitter)
	return s
}
