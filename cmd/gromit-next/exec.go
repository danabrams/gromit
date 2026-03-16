package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	storeDir      string
	stageProvider StageProvider
}

// run executes the spec pipeline and returns the formatted result string.
func (e *execSpecRun) run(ctx context.Context) (string, error) {
	// 1. Load execution policy
	var policy execpolicy.Policy
	var err error
	if e.policyPath != "" {
		policy, err = execpolicy.LoadPolicy(e.policyPath)
	} else {
		policy = execpolicy.DefaultPolicy()
	}
	if err != nil {
		return "", fmt.Errorf("load policy: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return "", fmt.Errorf("invalid policy: %w", err)
	}

	// 2. Create run state
	store := runstore.NewStore(e.storeDir)
	rs := runstore.NewRunState(specIDFromPath(e.specPath), e.projectID)

	// 3. Create a single shared Budget instance. This same instance is passed
	// to both the SpecLoop (for cycle counting and hard budget checks between
	// stages) and to ExecuteStage (for per-task cost accumulation). Using one
	// instance ensures cost tracked during task execution is visible to the
	// SpecLoop's budget gate.
	budget := specloop.NewBudget(policy.Budgets)

	// 3b. Create the event log so pipeline events are persisted to disk.
	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// 4. Build stages via provider, passing the shared budget and event log
	stages, err := e.stageProvider.BuildStages(policy, rs, budget, eventLog)
	if err != nil {
		return "", err
	}

	// 5. Filter for dry-run
	stages = filterStagesForDryRun(stages, e.dryRun)
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
		EventLog:    eventLog,
	})

	if err := loop.Run(ctx, rs); err != nil {
		return "", fmt.Errorf("spec loop: %w", err)
	}

	// 6. Persist final state
	if err := store.Save(rs); err != nil {
		return "", fmt.Errorf("save run state: %w", err)
	}

	// 7. Print terminal state and run ID
	return fmt.Sprintf("Run ID:  %s\nStatus:  %s\n", rs.RunID, rs.Status), nil
}

// newExecSpecCmd creates the `exec spec` command. Exported for testing.
func newExecSpecCmd() *cobra.Command {
	return newExecSpecCmdWithProvider(nil)
}

// newExecSpecCmdWithProvider creates the `exec spec` command with an explicit
// StageProvider. If provider is nil, the defaultStageProvider is used.
func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Execute a spec through the full pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, _ := cmd.Flags().GetString("spec")
			projectID, _ := cmd.Flags().GetString("project")
			policyPath, _ := cmd.Flags().GetString("policy")
			resolver := workspace.NewEnvResolver()
			root, _ := resolver.Resolve()
			if policyPath == "" && root != "" {
				policyPath = filepath.Join(root.ProjectCell(projectID), "policy", "execution.json")
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			p := provider
			if p == nil {
				workDir := resolveWorkDir(projectID, root)
				claudeClient, err := claude.NewClient("claude", []string{"--dangerously-skip-permissions"}, 300)
				if err != nil {
					return fmt.Errorf("create claude client: %w", err)
				}
				claudeProv := providerPkg.NewClaudeProvider(claudeClient, map[string]string{
					"low":    "haiku",
					"medium": "sonnet",
					"high":   "sonnet",
				})
				p = NewRealStageProvider(RealStageProviderConfig{
					WorkDir:        workDir,
					StoreDir:       storeDir,
					SpecPath:       specPath,
					PolicyPath:     policyPath,
					ClaudeProvider: claudeProv,
				})
			}

			r := &execSpecRun{
				specPath:      specPath,
				projectID:     projectID,
				policyPath:    policyPath,
				dryRun:        dryRun,
				storeDir:      storeDir,
				stageProvider: p,
			}

			output, err := r.run(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().String("spec", "", "Path to spec markdown file")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().String("policy", "", "Path to execution policy JSON file")
	cmd.Flags().Bool("dry-run", false, "Compile plan but do not execute")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	_ = cmd.MarkFlagRequired("spec")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}
