package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
	"github.com/danabrams/gromit/internal/next/validator"
	"github.com/danabrams/gromit/internal/provider"
)

// RealStageProviderConfig holds paths and options for building real stages.
type RealStageProviderConfig struct {
	WorkDir    string
	StoreDir   string
	SpecPath   string
	PolicyPath string
	Provider   provider.Provider // LLM provider (0002c: Claude only; 0002d: replaced by Router). Nil falls back to noops.
}

// RealStageProvider builds real stages using noop agents where LLM dependencies
// are not yet configured. This replaces the stub defaultStageProvider.
type RealStageProvider struct {
	cfg RealStageProviderConfig
}

// NewRealStageProvider creates a RealStageProvider.
func NewRealStageProvider(cfg RealStageProviderConfig) *RealStageProvider {
	return &RealStageProvider{cfg: cfg}
}

// BuildStages constructs the ordered pipeline of stages.
// The budget parameter is the single shared Budget instance created by exec.go;
// it is passed to ExecuteStage so that cost accumulated during task execution
// is visible to the SpecLoop's between-stage hard budget check.
func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget) ([]specloop.Stage, error) {
	// Validate replan threshold early (fail fast on invalid config).
	if _, err := review.ParseSeverity(policy.Review.ReplanThreshold); err != nil {
		return nil, fmt.Errorf("invalid replan threshold: %w", err)
	}

	// Read spec content for review and acceptance stages.
	specContent, err := os.ReadFile(p.cfg.SpecPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read spec file: %w", err)
	}

	store := runstore.NewStore(p.cfg.StoreDir)

	gitOps := &noopGitOps{workDir: p.cfg.WorkDir}

	initStage := stages.NewInitStage(stages.InitStageConfig{
		SpecPath:   p.cfg.SpecPath,
		PolicyPath: p.cfg.PolicyPath,
		RepoDir:    p.cfg.WorkDir,
		GitOps:     gitOps,
	}, store, nil)

	// Convert execpolicy.Check to validator.Check.
	alwaysRun := make([]validator.Check, len(policy.AlwaysRun))
	for i, c := range policy.AlwaysRun {
		alwaysRun[i] = validator.Check{Name: c.Name, Command: c.Command, Type: c.Type}
	}

	var (
		compiler      stages.SpecCompiler
		planCreator   stages.PlanCreator
		taskRunner    specloop.TaskRunner
		finalVal      stages.FinalValidator
		reviewRunner  stages.ReviewRunner
		acceptEval    stages.AcceptEvaluator
		diffProv      review.DiffProvider = &noopDiffProvider{}
	)

	if p.cfg.Provider != nil {
		// Wire real LLM-backed adapters.
		planAdapter := llmadapter.New(p.cfg.Provider, llmadapter.Config{
			Tier:   policy.Models.Planner,
			OnCost: func(cost float64) { budget.AddCost(cost) },
		})
		planAgent := planner.NewProviderPlanAgent(planAdapter)
		pl := planner.NewPlanner(planAgent, policy.Models.Planner)
		planCreator = pl

		// Execute adapter: OnCost is nil to avoid double-counting —
		// RunTaskLoop already calls Budget.AddCost(result.Cost) after each task.
		execAdapter := llmadapter.New(p.cfg.Provider, llmadapter.Config{
			Tier: policy.Models.Executor,
		})
		taskRunner = specloop.NewProviderTaskRunner(execAdapter)

		finalVal = validator.NewShellValidator(validator.NewRunner())

		reviewAdapter := llmadapter.New(p.cfg.Provider, llmadapter.Config{
			Tier:   policy.Models.Evaluator,
			OnCost: func(cost float64) { budget.AddCost(cost) },
		})
		reviewAgent := review.NewProviderReviewAgent(reviewAdapter)
		threshold, _ := review.ParseSeverity(policy.Review.ReplanThreshold)
		reviewRunner = review.NewRunner(reviewAgent, review.RunnerConfig{
			Facets:    policy.Review.Facets,
			Threshold: threshold,
			FacetTiers: policy.Review.Tiers,
		})

		acceptAdapter := llmadapter.New(p.cfg.Provider, llmadapter.Config{
			Tier:   policy.Models.Evaluator,
			OnCost: func(cost float64) { budget.AddCost(cost) },
		})
		acceptAgent := acceptor.NewProviderAcceptAgent(acceptAdapter)
		acceptEval = acceptor.NewEvaluator(acceptAgent)

		diffProv = &review.GitDiffProvider{WorkDir: p.cfg.WorkDir}

		compiler = &noopCompiler{} // TODO(0002d): wire SpecCompilerAdapter with real ArtifactStore
	} else {
		// Fallback to noops when no Provider is configured.
		compiler = &noopCompiler{}
		planCreator = &noopPlanCreator{}
		taskRunner = &noopTaskRunner{}
		finalVal = &noopValidator{}
		reviewRunner = &noopReviewRunner{}
		acceptEval = &noopAcceptEvaluator{}
	}

	compileStage := stages.NewCompileStage(compiler, store, nil)

	planStage := stages.NewPlanStage(planCreator, store, nil)
	if p.cfg.Provider != nil {
		// Planner satisfies both PlanCreator and FixPlanCreator.
		planStage.SetFixPlanner(planCreator.(stages.FixPlanCreator))
	}

	executeStage := stages.NewExecuteStage(taskRunner, stages.ExecuteStageConfig{
		MaxRetries:             policy.Budgets.MaxTaskRetries,
		MaxRedecompositions:    policy.Budgets.MaxRedecompositionPasses,
		WorkDir:                p.cfg.WorkDir,
		MaxTaskDurationSeconds: policy.Budgets.MaxTaskDurationSeconds,
		Budget:                 budget,
	})

	validateStage := stages.NewValidateStage(finalVal, stages.ValidateStageConfig{
		AlwaysRun: alwaysRun,
		WorkDir:   p.cfg.WorkDir,
	}, nil)

	reviewStage := stages.NewReviewStage(reviewRunner, stages.ReviewStageConfig{
		SpecContent:  string(specContent),
		DiffProvider: diffProv,
		BaseBranch:   "main",
		DefaultTier:  policy.Models.Evaluator,
		FacetTiers:   policy.Review.Tiers,
	}, nil)

	acceptStage := stages.NewAcceptStage(acceptEval, stages.AcceptStageConfig{
		SpecContent:  string(specContent),
		DiffProvider: diffProv,
		BaseBranch:   "main",
		Tier:         policy.Models.Evaluator,
	}, nil)

	evidenceStage := stages.NewEvidenceStage(store, stages.EvidenceStageConfig{
		DiffProvider: &noopDiffProvider{},
		BaseBranch:   "main",
		StartTime:    time.Now(),
	})

	finalizeStage := stages.NewFinalizeStage(gitOps, store, nil)

	return []specloop.Stage{
		initStage,
		compileStage,
		planStage,
		executeStage,
		validateStage,
		reviewStage,
		acceptStage,
		evidenceStage,
		finalizeStage,
	}, nil
}

// noopCompiler satisfies SpecCompiler with a no-op.
type noopCompiler struct{}

func (n *noopCompiler) Compile(_ context.Context) (string, error) {
	return "noop spec packet", nil
}

// noopPlanCreator satisfies PlanCreator with a no-op.
type noopPlanCreator struct{}

func (n *noopPlanCreator) CreatePlan(_ context.Context, _ planner.PlanRequest) (planner.Plan, error) {
	return planner.Plan{}, nil
}

// noopTaskRunner satisfies TaskRunner with a no-op.
type noopTaskRunner struct{}

func (n *noopTaskRunner) RunTask(_ context.Context, task runstore.Task) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

func (n *noopTaskRunner) RepairTask(_ context.Context, task runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

// noopGitOps satisfies GitOps with a no-op that uses a temp directory as worktree.
type noopGitOps struct {
	workDir string
}

func (n *noopGitOps) CreateWorktree(_, _ string) (string, error) {
	dir, err := os.MkdirTemp("", "gromit-noop-worktree-*")
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (n *noopGitOps) RemoveWorktree(path string) error {
	return os.RemoveAll(path)
}

// noopValidator satisfies FinalValidator with a no-op.
type noopValidator struct{}

func (n *noopValidator) RunFinal(_ context.Context, _ []validator.Check, _ []validator.Check, _ string) (validator.FinalResult, error) {
	return validator.FinalResult{Pass: true}, nil
}

// noopReviewRunner satisfies ReviewRunner with a no-op.
type noopReviewRunner struct{}

func (n *noopReviewRunner) Run(_ context.Context, _ review.RunInput) (*review.RunResult, error) {
	r := &review.RunResult{}
	r.NormalizeNilFields()
	return r, nil
}

// noopDiffProvider satisfies review.DiffProvider with a no-op.
type noopDiffProvider struct{}

func (n *noopDiffProvider) Diff(_ string) (string, error) {
	return "", nil
}

// noopAcceptEvaluator satisfies AcceptEvaluator with a no-op.
type noopAcceptEvaluator struct{}

func (n *noopAcceptEvaluator) Evaluate(_ context.Context, _ acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
	r := acceptor.AcceptanceResult{AllPass: true}
	r.NormalizeNilFields()
	return r, nil
}
