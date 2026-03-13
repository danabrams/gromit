package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
	"github.com/danabrams/gromit/internal/next/validator"
)

// RealStageProviderConfig holds paths and options for building real stages.
type RealStageProviderConfig struct {
	WorkDir    string
	StoreDir   string
	SpecPath   string
	PolicyPath string
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
func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState) ([]specloop.Stage, error) {
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

	compileStage := stages.NewCompileStage(&noopCompiler{}, store, nil)

	planStage := stages.NewPlanStage(&noopPlanCreator{}, store, nil)

	budget := specloop.NewBudget(policy.Budgets)
	executeStage := stages.NewExecuteStage(&noopTaskRunner{}, stages.ExecuteStageConfig{
		MaxRetries:             policy.Budgets.MaxTaskRetries,
		MaxRedecompositions:    policy.Budgets.MaxRedecompositionPasses,
		WorkDir:                p.cfg.WorkDir,
		MaxTaskDurationSeconds: policy.Budgets.MaxTaskDurationSeconds,
		Budget:                 budget,
	})

	// Convert execpolicy.Check to validator.Check.
	alwaysRun := make([]validator.Check, len(policy.AlwaysRun))
	for i, c := range policy.AlwaysRun {
		alwaysRun[i] = validator.Check{Name: c.Name, Command: c.Command, Type: c.Type}
	}
	validateStage := stages.NewValidateStage(&noopValidator{}, stages.ValidateStageConfig{
		AlwaysRun: alwaysRun,
		WorkDir:   p.cfg.WorkDir,
	}, nil)

	// TODO(next-phase): Wire EvidenceDir from store.RunEvidenceDir(runID) so
	// review.json and acceptance.json are written during real pipeline runs.
	// TODO(next-phase): Wire SpecContent from spec file so review/acceptance
	// prompts contain actual spec text when real agents replace noops.
	reviewStage := stages.NewReviewStage(&noopReviewRunner{}, stages.ReviewStageConfig{
		SpecContent:  string(specContent),
		DiffProvider: &noopDiffProvider{},
		BaseBranch:   "main",
		DefaultTier:  policy.Models.Evaluator,
		FacetTiers:   policy.Review.Tiers,
	}, nil)

	// TODO(next-phase): Wire DiffProvider and BaseBranch into AcceptStageConfig
	// so acceptance evaluation has diff context with real agents.
	acceptStage := stages.NewAcceptStage(&noopAcceptEvaluator{}, stages.AcceptStageConfig{
		SpecContent: string(specContent),
		Tier:        policy.Models.Evaluator,
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
