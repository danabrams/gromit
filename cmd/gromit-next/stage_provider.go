package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/projectcell"
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
	RepoDir        string // root of the git repo; falls back to WorkDir when empty
	StoreDir       string
	SpecPath       string
	PolicyPath     string
	ProjectsDir    string            // path to projects directory (for cell path resolution)
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

	gitOps := &realGitOps{}

	initStage := stages.NewInitStage(stages.InitStageConfig{
		SpecPath:   p.cfg.SpecPath,
		PolicyPath: p.cfg.PolicyPath,
		RepoDir:    p.cfg.WorkDir,
		GitOps:     gitOps,
	}, store, eventLog)

	// Convert execpolicy.Check to validator.Check.
	alwaysRun := make([]validator.Check, len(policy.AlwaysRun))
	for i, c := range policy.AlwaysRun {
		alwaysRun[i] = validator.Check{Name: c.Name, Command: c.Command, Type: c.Type}
	}
	autoFix := make([]validator.Check, len(policy.AutoFix))
	for i, c := range policy.AutoFix {
		autoFix[i] = validator.Check{Name: c.Name, Command: c.Command, Type: c.Type}
	}

	var (
		compiler           stages.SpecCompiler
		planCreator        stages.PlanCreator
		taskRunner         specloop.TaskRunner
		finalVal           stages.FinalValidator
		reviewRunner       stages.ReviewRunner
		acceptEval         stages.AcceptEvaluator
		contractWriter     contract.ContractWriter
		scenarioTestWriter contract.ScenarioTestWriter
		diffProv           review.DiffProvider = &noopDiffProvider{}
	)

	// Resolve cellPath for loading doctrine and playbook if ProjectsDir is available.
	cellPath := ""
	if p.cfg.ProjectsDir != "" && rs.ProjectID != "" {
		cellPathResolver := NewProjectCellPathResolver(p.cfg.ProjectsDir)
		resolved, err := cellPathResolver.ResolveCellPath(rs.ProjectID)
		if err == nil {
			cellPath = resolved
		}
	}

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
		workDirFn := func() string {
			if rs.WorktreePath != "" {
				return rs.WorktreePath
			}
			return p.cfg.WorkDir
		}
		ptr := specloop.NewProviderTaskRunner(execAdapter, workDirFn)
		ptr.SetContextProvider(specloop.FileTaskContextProvider(workDirFn, store.RunDir(rs.RunID), cellPath))
		taskRunner = ptr

		finalVal = validator.NewShellValidator(validator.NewRunner())

		// Review adapter with FallbackAdapter.
		reviewAdapter := llmadapter.NewFallbackAdapter(
			router, "review",
			llmadapter.Config{Tier: policy.Models.Evaluator, OnCost: costCallback, OnInvocation: invocationCallback},
			policy.Models.Evaluator,
		)
		reviewAgent := review.NewProviderReviewAgentWithDir(reviewAdapter, func() string {
			if rs.WorktreePath != "" {
				return rs.WorktreePath
			}
			return p.cfg.WorkDir
		})
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

		// Contract writer with FallbackAdapter (Sonnet/Planner tier).
		contractAdapter := llmadapter.NewFallbackAdapter(
			router, "contracts",
			llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback, OnInvocation: invocationCallback},
			policy.Models.Planner,
		)
		contractWriter = contract.NewLLMContractWriter(contractAdapter)

		// Scenario test writer with FallbackAdapter (Sonnet/Planner tier).
		scenarioTestAdapter := llmadapter.NewFallbackAdapter(
			router, "scenario_tests",
			llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback, OnInvocation: invocationCallback},
			policy.Models.Planner,
		)

		// Read scenario test patterns from docs/scenario-tests.md
		scenarioTestPatterns := ""
		patternsBase := p.cfg.RepoDir
		if patternsBase == "" {
			patternsBase = p.cfg.WorkDir
		}
		patternsPath := filepath.Join(patternsBase, "docs", "scenario-tests.md")
		patternsContent, err := os.ReadFile(patternsPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read scenario test patterns: %w", err)
		}
		if err == nil {
			scenarioTestPatterns = string(patternsContent)
		} else {
			log.Printf("warning: scenario test patterns file not found at %s, proceeding without pattern guidance", patternsPath)
		}

		scenarioTestWriter = contract.NewLLMScenarioTestWriter(scenarioTestAdapter, scenarioTestPatterns)

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
		contractWriter = &noopContractWriter{}
		scenarioTestWriter = &noopScenarioTestWriter{}
	}

	compileStage := stages.NewCompileStage(compiler, store, nil)

	planStage := stages.NewPlanStage(planCreator, store, nil)
	if p.claudeProvider != nil {
		// Planner satisfies both PlanCreator and FixPlanCreator.
		if fp, ok := planCreator.(stages.FixPlanCreator); ok {
			planStage.SetFixPlanner(fp)
		}
	}
	// Wire cell path resolver for playbook and doctrine loading
	if p.cfg.ProjectsDir != "" {
		cellPathResolver := NewProjectCellPathResolver(p.cfg.ProjectsDir)
		planStage.SetCellPathResolver(cellPathResolver)
	}
	planStage.SetStoreRootDir(p.cfg.StoreDir)

	var decomposer specloop.TaskDecomposer
	if p.claudeProvider != nil && policy.Budgets.MaxRedecompositionPasses > 0 {
		decomposer = &PlannerDecomposer{
			planner: planCreator,
			tier:    policy.Models.Planner,
		}
	}

	// ExecuteStage resolves rs.WorktreePath at runtime in Run().
	executeStage := stages.NewExecuteStage(taskRunner, stages.ExecuteStageConfig{
		MaxRetries:          policy.Budgets.MaxTaskRetries,
		MaxRedecompositions: policy.Budgets.MaxRedecompositionPasses,
		Inspector: specloop.NewShellTaskInspector(func() string {
			if rs.WorktreePath != "" {
				return rs.WorktreePath
			}
			return p.cfg.WorkDir
		}),
		Decomposer:             decomposer,
		GitOps:                 &shellGitOps{},
		WorkDir:                p.cfg.WorkDir,
		MaxTaskDurationSeconds: policy.Budgets.MaxTaskDurationSeconds,
		Budget:                 budget,
		DetectFilesChanged:     specloop.GitFilesChanged(),
		EventLog:               eventLog,
		CellPath:               cellPath,
	})

	evidenceDir := store.RunEvidenceDir(rs.RunID)

	writeScenarioTestsStage := stages.NewWriteScenarioTestsStage(scenarioTestWriter, stages.WriteScenarioTestsStageConfig{
		SpecPath:    p.cfg.SpecPath,
		EvidenceDir: evidenceDir,
		Store:       store,
		WorkDir:     p.cfg.WorkDir, // stage resolves rs.WorktreePath at runtime
		CompileDir:  rs.WorktreePath,
	}, budget, eventLog)

	contractEvaluator := &contract.DefaultContractEvaluator{}

	writeContractsStage := stages.NewWriteContractsStage(contractWriter, stages.WriteContractsStageConfig{
		SpecPath:    p.cfg.SpecPath,
		EvidenceDir: evidenceDir,
		Store:       store,
	}, budget, eventLog)

	repoDir := p.cfg.RepoDir
	if repoDir == "" {
		repoDir = p.cfg.WorkDir
	}

	// ValidateStage resolves rs.WorktreePath at runtime in Run().
	validateStage := stages.NewValidateStage(finalVal, stages.ValidateStageConfig{
		AlwaysRun:        alwaysRun,
		AutoFix:          autoFix,
		WorkDir:          p.cfg.WorkDir,
		EvidenceDir:      evidenceDir,
		RepoDir:          repoDir,
		SearchExtensions: []string{".go"},
		SpecText:         string(specContent),
	}, eventLog, contractEvaluator, gitOps)

	reviewStage := stages.NewReviewStage(reviewRunner, stages.ReviewStageConfig{
		SpecContent:  string(specContent),
		EvidenceDir:  evidenceDir,
		DiffProvider: diffProv,
		BaseBranch:   "main",
		DefaultTier:  policy.Models.Evaluator,
		FacetTiers:   policy.Review.Tiers,
	}, eventLog)

	acceptStage := stages.NewAcceptStage(acceptEval, stages.AcceptStageConfig{
		SpecContent:  string(specContent),
		EvidenceDir:  evidenceDir,
		DiffProvider: diffProv,
		BaseBranch:   "main",
		Tier:         policy.Models.Evaluator,
	}, eventLog)

	evidenceStage := stages.NewEvidenceStage(store, stages.EvidenceStageConfig{
		DiffProvider:     diffProv,
		BaseBranch:       "main",
		StartTime:        time.Now(),
		InvocationSource: budget,
	})

	finalizeStage := stages.NewFinalizeStageWithConfig(gitOps, store, nil, &stages.FinalizeStageConfig{
		SpecContent: string(specContent),
		EvidenceDir: evidenceDir,
	})

	// Build-time assertion: when a worktree is active, all stage WorkDir
	// values must point inside it.
	// All stages that use WorkDir resolve rs.WorktreePath at runtime in Run(),
	// so build-time worktree validation is no longer needed.

	allStages := []specloop.Stage{
		initStage,
		compileStage,
		planStage,
		writeContractsStage,
		executeStage,
		writeScenarioTestsStage,
		validateStage,
		reviewStage,
		acceptStage,
		evidenceStage,
		finalizeStage,
	}

	// Runtime guard: when a worktree is active, wrap every stage so that
	// any unexpected modification to the main repo is caught immediately.
	// Capture a baseline snapshot of the main repo's git status so that
	// pre-existing untracked files don't cause false-positive blocks.
	if rs.WorktreePath != "" {
		guardDir := p.cfg.RepoDir
		if guardDir == "" {
			guardDir = p.cfg.WorkDir
		}
		baseline, _ := specloop.DefaultGitStatus(guardDir)
		baselineSet := specloop.ParseStatusLines(baseline)
		for i, s := range allStages {
			allStages[i] = &specloop.WorktreeGuard{
				Inner:    s,
				RepoDir:  guardDir,
				Baseline: baselineSet,
			}
		}
	}

	return allStages, nil
}

// workDirEntry pairs a stage name with the WorkDir value it was constructed with.
type workDirEntry struct {
	stage string
	dir   string
}

// validateWorkDirsInWorktree returns an error if worktreePath is non-empty and
// any entry's dir does not start with worktreePath. This is a build-time safety
// net ensuring every stage that operates on the filesystem uses the worktree.
func validateWorkDirsInWorktree(worktreePath string, entries []workDirEntry) error {
	if worktreePath == "" {
		return nil
	}
	for _, wd := range entries {
		if !strings.HasPrefix(wd.dir, worktreePath) {
			return fmt.Errorf("stage %q WorkDir %q is outside worktree %q", wd.stage, wd.dir, worktreePath)
		}
	}
	return nil
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

// ProjectCellPathResolver implements stages.CellPathResolver using the project cell store.
type ProjectCellPathResolver struct {
	projectsDir string
}

// NewProjectCellPathResolver creates a new ProjectCellPathResolver.
func NewProjectCellPathResolver(projectsDir string) *ProjectCellPathResolver {
	return &ProjectCellPathResolver{projectsDir: projectsDir}
}

// ResolveCellPath returns the cell path for the given project ID.
func (r *ProjectCellPathResolver) ResolveCellPath(projectID string) (string, error) {
	if r.projectsDir == "" || projectID == "" {
		return "", nil
	}
	store := projectcell.NewFSStore(r.projectsDir)
	cell, err := store.Get(projectID)
	if err != nil {
		return "", nil
	}
	return cell.CellPath, nil
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
// falling back to the original WorkDir. WorktreePath is preferred because
// execution happens in the worktree when one is provisioned.
type lazyDiffProvider struct {
	rs          *runstore.RunState
	fallbackDir string
}

func (l *lazyDiffProvider) Diff(baseBranch string) (string, error) {
	// Prefer WorktreePath (where execution happens) over fallbackDir.
	dir := l.rs.WorktreePath
	if dir == "" {
		dir = l.fallbackDir
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

// noopContractWriter satisfies contract.ContractWriter with a no-op.
type noopContractWriter struct{}

func (n *noopContractWriter) WriteContracts(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
	return nil, nil
}

// noopScenarioTestWriter satisfies contract.ScenarioTestWriter with a no-op.
type noopScenarioTestWriter struct{}

func (n *noopScenarioTestWriter) WriteScenarioTest(_ context.Context, _ contract.SpecScenario, _ []string, _ string, _ string) (string, error) {
	return "", nil
}
