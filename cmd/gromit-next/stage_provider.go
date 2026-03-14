package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	WorkDir        string
	StoreDir       string
	SpecPath       string
	PolicyPath     string
	Provider       provider.Provider // LLM provider (0002c: Claude only; 0002d: replaced by Router). Nil falls back to noops.
	ClaudeProvider provider.Provider
	CodexProvider  provider.Provider
	StateFn        provider.StateFile
	CircuitBreaker *provider.CircuitBreaker
}

// RealStageProvider builds real stages using noop agents where LLM dependencies
// are not yet configured. This replaces the stub defaultStageProvider.
type RealStageProvider struct {
	cfg            RealStageProviderConfig
	claudeProvider provider.Provider
	codexProvider  provider.Provider
	stateFn        provider.StateFile
	circuitBreaker *provider.CircuitBreaker
}

// NewRealStageProvider creates a RealStageProvider.
// If ClaudeProvider is nil but the legacy Provider field is set, the legacy
// provider is promoted to claudeProvider so that all LLM wiring flows through
// the single FallbackAdapter code path.
func NewRealStageProvider(cfg RealStageProviderConfig) *RealStageProvider {
	claude := cfg.ClaudeProvider
	if claude == nil && cfg.Provider != nil {
		claude = cfg.Provider
	}
	return &RealStageProvider{
		cfg:            cfg,
		claudeProvider: claude,
		codexProvider:  cfg.CodexProvider,
		stateFn:        cfg.StateFn,
		circuitBreaker: cfg.CircuitBreaker,
	}
}

// BuildStages constructs the ordered pipeline of stages.
// The budget parameter is the single shared Budget instance created by exec.go;
// it is passed to ExecuteStage so that cost accumulated during task execution
// is visible to the SpecLoop's between-stage hard budget check.
func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	// Validate and parse replan threshold early (fail fast on invalid config).
	threshold, err := review.ParseSeverity(policy.Review.ReplanThreshold)
	if err != nil {
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
		compiler     stages.SpecCompiler
		planCreator  stages.PlanCreator
		taskRunner   specloop.TaskRunner
		finalVal     stages.FinalValidator
		reviewRunner stages.ReviewRunner
		acceptEval   stages.AcceptEvaluator
		diffProv     review.DiffProvider = &noopDiffProvider{}
	)

	if p.claudeProvider != nil {
		router := p.buildRouter(policy)
		costCallback := func(c float64) { budget.AddCost(c) }
		invocationCallback := func(r runstore.InvocationRecord) { budget.AddInvocation(r) }

		// Plan adapter with FallbackAdapter for lazy provider selection.
		planAdapter := llmadapter.NewFallbackAdapter(
			router, "plan",
			llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback, OnInvocation: invocationCallback},
			policy.Models.Planner,
		)
		planAgent := planner.NewProviderPlanAgent(planAdapter, policy.Models.Planner)
		pl := planner.NewPlanner(planAgent, policy.Models.Planner)
		planCreator = pl

		// Execute adapter: OnCost is intentionally nil to avoid double-counting.
		// RunTaskLoop already calls Budget.AddCost(result.Cost) after each task,
		// so wiring OnCost here would count execution costs twice.
		// OnInvocation has no such double-counting risk and is wired normally.
		execAdapter := llmadapter.NewFallbackAdapter(
			router, "execute",
			llmadapter.Config{Tier: policy.Models.Executor, OnInvocation: invocationCallback},
			policy.Models.Executor,
		)
		taskRunner = specloop.NewProviderTaskRunner(execAdapter, p.cfg.WorkDir)

		finalVal = validator.NewShellValidator(validator.NewRunner())

		// Review adapter with FallbackAdapter.
		reviewAdapter := llmadapter.NewFallbackAdapter(
			router, "review",
			llmadapter.Config{Tier: policy.Models.Evaluator, OnCost: costCallback, OnInvocation: invocationCallback},
			policy.Models.Evaluator,
		)
		reviewAgent := review.NewProviderReviewAgent(reviewAdapter)
		reviewRunner = review.NewRunner(reviewAgent, review.RunnerConfig{
			Facets:     policy.Review.Facets,
			Threshold:  threshold,
			FacetTiers: policy.Review.Tiers,
		})

		// Accept adapter with FallbackAdapter.
		acceptAdapter := llmadapter.NewFallbackAdapter(
			router, "accept",
			llmadapter.Config{Tier: policy.Models.Evaluator, OnCost: costCallback, OnInvocation: invocationCallback},
			policy.Models.Evaluator,
		)
		acceptAgent := acceptor.NewProviderAcceptAgent(acceptAdapter)
		acceptEval = acceptor.NewEvaluator(acceptAgent)

		diffProv = &lazyDiffProvider{rs: rs, fallbackDir: p.cfg.WorkDir}

		// TODO: Wire real SpecCompilerAdapter here (blocked on ArtifactStore, cell resolution, level selection).
		// For now, pass-through the raw spec file as the spec packet.
		compiler = &passthruCompiler{specPath: p.cfg.SpecPath}
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
	if p.claudeProvider != nil {
		// Planner satisfies both PlanCreator and FixPlanCreator.
		if fp, ok := planCreator.(stages.FixPlanCreator); ok {
			planStage.SetFixPlanner(fp)
		}
	}

	var decomposer specloop.TaskDecomposer
	if p.claudeProvider != nil && policy.Budgets.MaxRedecompositionPasses > 0 {
		decomposer = &PlannerDecomposer{
			planner: planCreator,
			tier:    policy.Models.Planner,
		}
	}

	executeStage := stages.NewExecuteStage(taskRunner, stages.ExecuteStageConfig{
		MaxRetries:             policy.Budgets.MaxTaskRetries,
		MaxRedecompositions:    policy.Budgets.MaxRedecompositionPasses,
		Inspector:              specloop.NewShellTaskInspector(p.cfg.WorkDir),
		Decomposer:             decomposer,
		GitOps:                 &shellGitOps{},
		WorkDir:                p.cfg.WorkDir,
		MaxTaskDurationSeconds: policy.Budgets.MaxTaskDurationSeconds,
		Budget:                 budget,
		DetectFilesChanged:     specloop.GitFilesChanged(),
		EventLog:               eventLog,
	})

	validateStage := stages.NewValidateStage(finalVal, stages.ValidateStageConfig{
		AlwaysRun: alwaysRun,
		WorkDir:   p.cfg.WorkDir,
	}, nil)

	evidenceDir := store.RunEvidenceDir(rs.RunID)

	reviewStage := stages.NewReviewStage(reviewRunner, stages.ReviewStageConfig{
		SpecContent:  string(specContent),
		EvidenceDir:  evidenceDir,
		DiffProvider: diffProv,
		BaseBranch:   "main",
		DefaultTier:  policy.Models.Evaluator,
		FacetTiers:   policy.Review.Tiers,
	}, nil)

	acceptStage := stages.NewAcceptStage(acceptEval, stages.AcceptStageConfig{
		SpecContent:  string(specContent),
		EvidenceDir:  evidenceDir,
		DiffProvider: diffProv,
		BaseBranch:   "main",
		Tier:         policy.Models.Evaluator,
	}, nil)

	evidenceStage := stages.NewEvidenceStage(store, stages.EvidenceStageConfig{
		DiffProvider:     diffProv,
		BaseBranch:       "main",
		StartTime:        time.Now(),
		InvocationSource: budget,
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

// buildRouter constructs a provider.Router from the policy's routing config
// and the provider fields on RealStageProvider.
func (p *RealStageProvider) buildRouter(policy execpolicy.Policy) *provider.Router {
	providers := map[string]provider.Provider{
		"claude": p.claudeProvider,
	}
	if p.codexProvider != nil {
		providers["codex"] = p.codexProvider
	}

	preferences := policy.Routing.Preferences
	ratio := policy.Routing.Ratio
	cooldown := time.Duration(policy.Routing.CooldownSeconds) * time.Second

	return provider.NewRouter(providers, preferences, ratio, cooldown, p.stateFn, p.circuitBreaker)
}

// noopCompiler satisfies SpecCompiler with a no-op.
type noopCompiler struct{}

func (n *noopCompiler) Compile(_ context.Context) (string, error) {
	return "noop spec packet", nil
}

// passthruCompiler reads the spec file and returns its content as the spec packet.
// This is a temporary stand-in until the real SpecCompilerAdapter is wired.
type passthruCompiler struct {
	specPath string
}

func (c *passthruCompiler) Compile(_ context.Context) (string, error) {
	data, err := os.ReadFile(c.specPath)
	if err != nil {
		return "", fmt.Errorf("read spec file: %w", err)
	}
	return string(data), nil
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

// noopGitOps satisfies GitOps with a copy-based worktree (no real git worktree).
// It copies the repo directory into a temp dir so the executor can work with
// real files. This is a stand-in until real git worktree support is wired.
type noopGitOps struct {
	workDir string
}

func (n *noopGitOps) CreateWorktree(repoDir, _ string) (string, error) {
	dir, err := os.MkdirTemp("", "gromit-noop-worktree-*")
	if err != nil {
		return "", err
	}
	// Copy repo contents so the executor has real files to work with.
	cmd := exec.Command("cp", "-a", repoDir+"/.", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("copy repo to worktree: %s: %w", string(out), err)
	}
	return dir, nil
}

func (n *noopGitOps) RemoveWorktree(path string) error {
	return os.RemoveAll(path)
}

// shellGitOps implements specloop.GitOps using git CLI commands.
// It is used by the task loop to revert files before redecomposition.
type shellGitOps struct{}

func (s *shellGitOps) CheckoutFiles(workDir string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"checkout", "--"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout files: %s: %w", string(out), err)
	}
	return nil
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

// lazyDiffProvider resolves WorkDir at call time from RunState.WorktreePath,
// falling back to the original WorkDir. Currently the executor runs in the
// original WorkDir (noopGitOps copies files but doesn't redirect execution),
// so we prefer fallbackDir until real git worktree support redirects execution
// into WorktreePath.
type lazyDiffProvider struct {
	rs          *runstore.RunState
	fallbackDir string
}

func (l *lazyDiffProvider) Diff(baseBranch string) (string, error) {
	// Prefer fallbackDir (where the executor actually runs) over WorktreePath
	// (the noopGitOps copy that never gets modified). When real git worktrees
	// are wired and execution happens in WorktreePath, swap priority here.
	dir := l.fallbackDir
	if dir == "" {
		dir = l.rs.WorktreePath
	}
	return (&review.GitDiffProvider{WorkDir: dir}).Diff(baseBranch)
}

// PlannerDecomposer implements specloop.TaskDecomposer by asking the planner
// to create a sub-plan for a task that is too broad to execute as-is.
type PlannerDecomposer struct {
	planner stages.PlanCreator
	tier    string
}

// Decompose invokes the planner to break the given task into smaller sub-tasks.
// It uses the task's objective as the spec packet so the planner can generate
// a focused sub-plan. The resulting tasks are returned as pending runstore.Task
// values ready to be enqueued by the task loop.
func (d *PlannerDecomposer) Decompose(ctx context.Context, task runstore.Task) ([]runstore.Task, error) {
	req := planner.PlanRequest{
		SpecPacket: "Decompose this task into smaller sub-tasks:\n\n" + task.Objective,
		Cycle:      task.Cycle,
	}
	plan, err := d.planner.CreatePlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("decompose task %s: %w", task.TaskID, err)
	}
	subTasks := make([]runstore.Task, len(plan.Tasks))
	for i, td := range plan.Tasks {
		st := runstore.Task{
			TaskID:              td.TaskID,
			Objective:           td.Objective,
			Status:              "pending",
			ExpectedTouchedArea: td.ExpectedTouchedArea,
			ProofChecks:         td.ProofChecks,
			Cycle:               task.Cycle,
			Kind:                "decomposed",
		}
		st.NormalizeNilFields()
		subTasks[i] = st
	}
	return subTasks, nil
}

// noopAcceptEvaluator satisfies AcceptEvaluator with a no-op.
type noopAcceptEvaluator struct{}

func (n *noopAcceptEvaluator) Evaluate(_ context.Context, _ acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
	r := acceptor.AcceptanceResult{AllPass: true}
	r.NormalizeNilFields()
	return r, nil
}
