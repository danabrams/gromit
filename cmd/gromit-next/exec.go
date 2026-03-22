package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/projectcell"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/workspace"
	providerPkg "github.com/danabrams/gromit/internal/provider"
	"github.com/spf13/cobra"
)

// resolveWorkDir returns the working directory for a spec execution.
// When projectID is set and the project cell exists, it returns cell.RepoPath
// so the executor always runs in the project's repo regardless of CWD.
// Falls back to os.Getwd() when projectID is empty or the cell is not found.
func resolveWorkDir(projectID string, root workspace.Root) string {
	if projectID != "" {
		store := projectcell.NewFSStore(root.ProjectsDir())
		if cell, err := store.Get(projectID); err == nil && cell.RepoPath != "" {
			return cell.RepoPath
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execution commands",
}

// specIDFromPath extracts the filename stem (without directory or extension)
// from a spec path, so that SpecID is always a bare name like "add-refund-endpoint"
// regardless of whether the user passes "./specs/add-refund-endpoint.md" or just
// "add-refund-endpoint".
func specIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// dryRunStages is the set of stage names that run during a dry-run.
var dryRunStages = map[string]bool{
	"init":    true,
	"compile": true,
	"plan":    true,
}

// filterStagesForResume returns stages with compile removed,
// since that stage has already been completed in the prior run.
// When worktreePath is non-empty, init is also skipped because the worktree
// already exists from the prior run.
// Other stages run idempotently based on their corresponding completion flags.
func filterStagesForResume(stages []specloop.Stage, worktreePath string) []specloop.Stage {
	var filtered []specloop.Stage
	for _, s := range stages {
		switch s.Name() {
		case "compile":
			continue
		case "init":
			if worktreePath != "" {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	return filtered
}

// filterStagesForDryRun returns only the dry-run stages when dryRun is true,
// or all stages when dryRun is false.
func filterStagesForDryRun(stages []specloop.Stage, dryRun bool) []specloop.Stage {
	if !dryRun {
		return stages
	}
	var filtered []specloop.Stage
	for _, s := range stages {
		if dryRunStages[s.Name()] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// StageProvider builds the ordered set of stages for an exec spec run.
// Implementations wire real or test dependencies into each stage.
// The Budget parameter is the single shared instance that tracks cost and time
// across both the SpecLoop (cycle counting, hard budget checks between stages)
// and the task loop inside ExecuteStage (per-task cost accumulation).
type StageProvider interface {
	BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error)
}

// execSpecRun holds the wiring for an exec spec invocation, separated from
// cobra for testability.
type execSpecRun struct {
	specPath      string
	projectID     string
	policyPath    string
	dryRun        bool
	resumeRunID   string
	resumeCycles  int
	storeDir      string
	stageProvider StageProvider
	policy        *execpolicy.Policy // pre-loaded policy; skips re-load in run()
	out           io.Writer
	store         *runstore.Store
}

// normalizeNilFields sets nil fields to safe defaults so run() never panics on
// a partially-constructed struct (e.g. in tests that only set some fields).
func (e *execSpecRun) normalizeNilFields() {
	if e.out == nil {
		e.out = io.Discard
	}
	if e.store == nil && e.storeDir != "" {
		e.store = runstore.NewStore(e.storeDir)
	}
}

// run executes the spec pipeline, writing progress and the terminal summary to e.out.
func (e *execSpecRun) run(ctx context.Context) error {
	e.normalizeNilFields()
	// 1. Load execution policy (skip if pre-loaded by caller)
	var policy execpolicy.Policy
	if e.policy != nil {
		policy = *e.policy
	} else {
		var err error
		if e.policyPath != "" {
			policy, err = execpolicy.LoadPolicy(e.policyPath)
		} else {
			policy = execpolicy.DefaultPolicy()
		}
		if err != nil {
			return fmt.Errorf("load policy: %w", err)
		}
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	// 2. Create or load run state
	var rs *runstore.RunState
	if e.resumeRunID != "" {
		var err error
		rs, err = e.store.Get(e.resumeRunID)
		if err != nil {
			return fmt.Errorf("load run for resume: %w", err)
		}
		// Mark as resumed so stages can skip redundant work
		rs.Resumed = true
		// Reset terminal state so the pipeline can re-run
		rs.Status = runstore.StatusRunning
		rs.TerminalReason = ""
		rs.BlockerSummary = ""
		rs.EndedAt = time.Time{}
		// Reset per-cycle gate flags
		rs.FinalValidationPassed = false
		rs.FinalReviewPassed = false
		rs.FinalAcceptancePassed = false
		rs.ReviewFindings = nil
		rs.AcceptanceResults = nil
		// Increment cycle for the resumed run
		rs.Cycle++
	} else {
		rs = runstore.NewRunState(specIDFromPath(e.specPath), e.projectID)
	}

	// 3. Create a single shared Budget instance. This same instance is passed
	// to both the SpecLoop (for cycle counting and hard budget checks between
	// stages) and to ExecuteStage (for per-task cost accumulation). Using one
	// instance ensures cost tracked during task execution is visible to the
	// SpecLoop's budget gate.
	// On resume, override MaxSpecCycles with the requested cycle count.
	if e.resumeRunID != "" && e.resumeCycles > 0 {
		policy.Budgets.MaxSpecCycles = e.resumeCycles
	}
	budget := specloop.NewBudget(policy.Budgets)

	// 3b. Create the event log so pipeline events are persisted to disk.
	eventLogPath := filepath.Join(e.store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// 3c. Write the start banner
	fmt.Fprintf(e.out, "Run ID: %s\nEvents: %s\n\n", rs.RunID, eventLogPath)

	// 4. Build stages via provider, passing the shared budget and event log
	stages, err := e.stageProvider.BuildStages(policy, rs, budget, eventLog)
	if err != nil {
		return err
	}

	// 5. Filter for dry-run or resume
	stages = filterStagesForDryRun(stages, e.dryRun)
	if e.resumeRunID != "" {
		stages = filterStagesForResume(stages, rs.WorktreePath)
	}
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
		EventLog:    eventLog,
	})

	if err := loop.Run(ctx, rs); err != nil {
		return fmt.Errorf("spec loop: %w", err)
	}

	// 6. Persist final state
	if err := e.store.Save(rs); err != nil {
		return fmt.Errorf("save run state: %w", err)
	}

	// 7. Print terminal state and run ID
	fmt.Fprintf(e.out, "Run ID:  %s\nStatus:  %s\n", rs.RunID, rs.Status)
	if rs.WorktreePath != "" {
		fmt.Fprintf(e.out, "Worktree: %s\n", rs.WorktreePath)
		fmt.Fprintf(e.out, "Branch:   %s\n", branchResolverFunc(rs.WorktreePath))
	}
	return nil
}

// newExecSpecCmd creates the `exec spec` command. Exported for testing.
func newExecSpecCmd() *cobra.Command {
	return newExecSpecCmdWithProvider(nil)
}

// branchResolverFunc resolves the git branch for a spec by running git symbolic-ref.
// Falls back to showing the abbreviated SHA for detached HEADs.
func branchResolverFunc(repoPath string) string {
	// Try: git -C <repoPath> symbolic-ref --short HEAD
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	// Detached HEAD — fall back to abbreviated SHA.
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--short", "HEAD")
	out, err = cmd.Output()
	if err == nil {
		return "detached:" + strings.TrimSpace(string(out))
	}
	return "(unknown)"
}

// pickSpec discovers specs, derives their status, filters to ready/ready_for_review,
// prompts the user to select one, and returns the full path to the selected spec.
func pickSpec(project, specsDir string, store *runstore.Store, branchResolver func(worktreePath string) string, in io.Reader, out io.Writer) (string, error) {
	// Discover all specs
	specs, err := DiscoverSpecs(specsDir)
	if err != nil {
		return "", err
	}

	// List runs for this project
	runs, err := store.List(project)
	if err != nil {
		return "", err
	}

	// Convert []*RunState to []RunState for DeriveSpecStatus
	runValues := make([]runstore.RunState, len(runs))
	for i, r := range runs {
		runValues[i] = *r
	}

	// Filter specs to those with status ready or ready_for_review
	type specInfo struct {
		name    string
		status  string
		runs    []runstore.RunState
		lastRun *runstore.RunState
	}

	var availableSpecs []specInfo

	for _, spec := range specs {
		// Filter runs for this spec
		var specRuns []runstore.RunState
		for _, r := range runValues {
			if r.SpecID == spec {
				specRuns = append(specRuns, r)
			}
		}

		// Read spec content to derive status
		content, _ := os.ReadFile(filepath.Join(specsDir, spec+".md"))
		status := DeriveSpecStatusFromContent(spec, specRuns, string(content))

		// Filter to ready or ready_for_review
		if status != "ready" && status != "ready_for_review" {
			continue
		}

		// Find the most recent run matching this spec's derived status.
		// For ready_for_review specs we need the ready_for_review run specifically,
		// not a later run that may have a different status (e.g. blocked).
		var lastRun *runstore.RunState
		if len(specRuns) > 0 {
			var latest *runstore.RunState
			for i := range specRuns {
				if specRuns[i].Status != status {
					continue
				}
				if latest == nil || specRuns[i].StartedAt.After(latest.StartedAt) {
					latest = &specRuns[i]
				}
			}
			// Fall back to most recent run overall if no status-matched run found
			if latest == nil {
				best := specRuns[0]
				for i := 1; i < len(specRuns); i++ {
					if specRuns[i].StartedAt.After(best.StartedAt) {
						best = specRuns[i]
					}
				}
				latest = &best
			}
			lastRun = latest
		}

		availableSpecs = append(availableSpecs, specInfo{
			name:    spec,
			status:  status,
			runs:    specRuns,
			lastRun: lastRun,
		})
	}

	// Check if any specs are available
	if len(availableSpecs) == 0 {
		fmt.Fprintf(out, "no specs available to run\n")
		return "", nil
	}

	// Print numbered list of available specs
	for i, spec := range availableSpecs {
		num := i + 1
		marker := ""
		extra := ""

		if spec.status == "ready_for_review" {
			marker = " * (ready_for_review)"
			if spec.lastRun != nil && spec.lastRun.WorktreePath != "" {
				wtLabel := spec.lastRun.WorktreePath
				if _, err := os.Stat(spec.lastRun.WorktreePath); err != nil {
					wtLabel += " (removed)"
				}
				branch := branchResolver(spec.lastRun.WorktreePath)
				extra = fmt.Sprintf("\n     worktree: %s\n     branch:   %s", wtLabel, branch)
			}
		}

		fmt.Fprintf(out, "%d. %s%s%s\n", num, spec.name, marker, extra)
	}

	// Read selection from stdin
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "\nEnter spec number: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("failed to read input")
	}

	selection := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(selection)
	if err != nil {
		return "", fmt.Errorf("invalid selection: %q", selection)
	}

	if num < 1 || num > len(availableSpecs) {
		return "", fmt.Errorf("selection out of range: %d", num)
	}

	selectedSpec := availableSpecs[num-1].name
	return filepath.Join(specsDir, selectedSpec+".md"), nil
}

// statusLabel returns a human-readable label for a run status.
func statusLabel(status string) string {
	switch status {
	case runstore.StatusRunning:
		return "running"
	case runstore.StatusReadyForReview:
		return "ready_for_review"
	case runstore.StatusNeedsHuman:
		return "needs_attention"
	case runstore.StatusBlocked:
		return "blocked"
	default:
		return status
	}
}

// pickRun lists runs for a project, filters to resumable statuses,
// prompts the user to select one, and returns the RunID.
func pickRun(project string, store *runstore.Store, in io.Reader, out io.Writer) (string, error) {
	// List all runs for this project
	runs, err := store.List(project)
	if err != nil {
		return "", err
	}

	// Filter to resumable statuses
	resumable := []runstore.RunState{}
	for _, r := range runs {
		switch r.Status {
		case runstore.StatusRunning, runstore.StatusNeedsHuman, runstore.StatusBlocked, runstore.StatusReadyForReview:
			resumable = append(resumable, *r)
		}
	}

	// Check if any runs are available
	if len(resumable) == 0 {
		fmt.Fprintf(out, "no runs available to resume\n")
		return "", nil
	}

	// Sort by StartedAt descending (most recent first)
	sort.Slice(resumable, func(i, j int) bool {
		return resumable[j].StartedAt.Before(resumable[i].StartedAt)
	})

	// Print numbered list with spec ID, status label, and timestamp
	for i, run := range resumable {
		num := i + 1
		label := statusLabel(run.Status)
		timestamp := run.StartedAt.Format("2006-01-02 15:04:05")
		fmt.Fprintf(out, "%d. %s [%s] %s\n", num, run.SpecID, label, timestamp)
	}

	// Read selection from stdin
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "\nEnter run number: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("failed to read input")
	}

	selection := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(selection)
	if err != nil {
		return "", fmt.Errorf("invalid selection: %q", selection)
	}

	if num < 1 || num > len(resumable) {
		return "", fmt.Errorf("selection out of range: %d", num)
	}

	return resumable[num-1].RunID, nil
}

// newExecSpecCmdWithProvider creates the `exec spec` command with an explicit
// StageProvider. If provider is nil, the defaultStageProvider is used.
func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Execute a spec through the full pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			specPath, _ := cmd.Flags().GetString("spec")
			projectID, _ := cmd.Flags().GetString("project")
			policyPath, _ := cmd.Flags().GetString("policy")
			specsDir, _ := cmd.Flags().GetString("specs-dir")
			resolver := workspace.NewEnvResolver()
			root, _ := resolver.Resolve()
			if policyPath == "" && root != "" {
				policyPath = filepath.Join(root.ProjectCell(projectID), "policy", "execution.json")
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			resumeRunID, _ := cmd.Flags().GetString("resume")
			// Fixup: cobra NoOptDefVal means `--resume run-xxx` (space-separated)
			// parses as --resume="__pick__" with "run-xxx" left as a positional arg.
			// Recover the intended run ID from args so both forms work identically.
			if resumeRunID == "__pick__" && len(args) > 0 && strings.HasPrefix(args[0], "run-") {
				resumeRunID = args[0]
				args = args[1:]
			}
			resumeCycles, _ := cmd.Flags().GetInt("cycles")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			// Construct store early for picker operations
			storeInstance := runstore.NewStore(storeDir)

			// Load policy early so budget values are available for client config.
			var policy execpolicy.Policy
			if policyPath != "" {
				var pErr error
				policy, pErr = execpolicy.LoadPolicy(policyPath)
				if pErr != nil {
					return fmt.Errorf("load policy: %w", pErr)
				}
			} else {
				policy = execpolicy.DefaultPolicy()
			}

			// Wire picker flow
			if resumeRunID == "__pick__" {
				runID, err := pickRun(projectID, storeInstance, cmd.InOrStdin(), cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("pick run: %w", err)
				}
				if runID == "" {
					return nil
				}
				resumeRunID = runID
			}

			if resumeRunID == "" && specPath == "" {
				// Resolve specsDir if not provided via flag
				if specsDir == "" {
					if root != "" {
						projectDir, err := ResolveProjectConfigPath(root, projectID)
						if err != nil {
							return fmt.Errorf("resolve project config: %w", err)
						}
						cfg, err := LoadProjectConfig(projectDir)
						if err != nil {
							return fmt.Errorf("load project config: %w", err)
						}
						specsDir = cfg.SpecsDir
						if specsDir == "" && cfg.RepoPath != "" {
							specsDir = filepath.Join(cfg.RepoPath, "docs", "specs")
						}
					}
					if specsDir == "" {
						return fmt.Errorf("cannot resolve specs directory: project config has no specs_dir or repo_path")
					}
				}
				selectedPath, err := pickSpec(projectID, specsDir, storeInstance, branchResolverFunc, cmd.InOrStdin(), cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("pick spec: %w", err)
				}
				if selectedPath == "" {
					return nil
				}
				specPath = selectedPath
			}

			// On resume, resolve specPath from the run's stored spec if not provided.
			if resumeRunID != "" && specPath == "" {
				stored := filepath.Join(storeDir, "runs", resumeRunID, "spec.md")
				if _, err := os.Stat(stored); err == nil {
					specPath = stored
				}
			}

			// Construct RealStageProvider after pickers
			p := provider
			if p == nil {
				workDir := resolveWorkDir(projectID, root)
				claudeClient, err := claude.NewClient("claude", []string{"--dangerously-skip-permissions"}, policy.Budgets.MaxTaskDurationSeconds)
				if err != nil {
					return fmt.Errorf("create claude client: %w", err)
				}
				tierModels := policy.Models.TierModels
				if len(tierModels) == 0 {
					tierModels = map[string]string{"low": "haiku", "medium": "sonnet", "high": "opus"}
				}
				claudeProv := providerPkg.NewClaudeProvider(claudeClient, tierModels)
				p = NewRealStageProvider(RealStageProviderConfig{
					WorkDir:        workDir,
					StoreDir:       storeDir,
					SpecPath:       specPath,
					PolicyPath:     policyPath,
					ProjectsDir:    filepath.Join(storeDir, "projects"),
					ClaudeProvider: claudeProv,
				})
			}

			r := &execSpecRun{
				specPath:      specPath,
				projectID:     projectID,
				policyPath:    policyPath,
				dryRun:        dryRun,
				resumeRunID:   resumeRunID,
				resumeCycles:  resumeCycles,
				storeDir:      storeDir,
				stageProvider: p,
				policy:        &policy,
				out:           cmd.OutOrStdout(),
				store:         storeInstance,
			}

			return r.run(cmd.Context())
		},
	}
	cmd.Flags().String("spec", "", "Path to spec markdown file")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().String("policy", "", "Path to execution policy JSON file")
	cmd.Flags().Bool("dry-run", false, "Compile plan but do not execute")
	cmd.Flags().String("resume", "", "Resume a previous run by run ID")
	cmd.Flag("resume").NoOptDefVal = "__pick__"
	cmd.Flags().Int("cycles", 0, "Number of cycles to run (useful with --resume)")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	cmd.Flags().String("specs-dir", "", "Override specs directory (for testing)")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}
