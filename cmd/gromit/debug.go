package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runbook"
	"github.com/danabrams/gromit/skills"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug [description]",
	Short: "Launch interactive bug investigation session",
	Long: `Launch an interactive agent session for investigating bugs.

The session receives full project context (CLAUDE.md, RULES.md, LEARNINGS.md)
and the debug workflow runs three jobs:
  - Diagnose: Summarize failure evidence, scope the problem, and explain the root cause.
  - Fix: Apply or plan a correction, choosing between trivial fixes, clear fixes, or further investigation.
  - Learn: Capture lessons, distill rule updates, and create backlog items so the same issues stay visible.

The session also guides free-form investigation to identify root cause, triage severity,
and produce appropriate outcomes:
  - Trivial fix: Apply directly and validate
  - Clear fix: Create investigation report + plan
  - Needs investigation: Create report + backlog item

Examples:
  gromit debug                              # Blank session
  gromit debug "login fails with + in email" # Pre-seeded description
  gromit debug --model sonnet "API returns 500" # Override model
  gromit debug --agent codex "Cache miss flapping" # Use Codex agent`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDebug,
}

var debugModel string

const (
	debugModelFlag       = "model"
	debugAgentFlag       = "agent"
	debugChooseAgentFlag = "choose-agent"
	debugRestoreFlag     = "restore"
	debugSessionCommand  = "debug"
	debugKeepPrompt      = "Keep worktree?"
	debugWorktreesDir    = "worktrees"
	debugWorktreePrefix  = "debug-"
	gitWorktreeCmd       = "worktree"
	gitWorktreeAddCmd    = "add"
	gitWorktreeRemoveCmd = "remove"
	gitDetachFlag        = "--detach"
	claudeAgentName      = "claude"
)

type debugGitRunFn func(dir string, args ...string) (string, error)

var debugGitRun debugGitRunFn = runDebugGit
var debugConfirmPromptFn = confirmPrompt
var debugSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var debugWarnModelFn = maybeWarnModelFlagOnNonClaudeAgent

func init() {
	debugCmd.Flags().StringVar(&debugModel, debugModelFlag, "opus", "Model to use when the Claude agent is selected (opus, sonnet, haiku)")
	debugCmd.Flags().String(debugAgentFlag, "", "Override the default agent for this debug session")
	debugCmd.Flags().Bool(debugChooseAgentFlag, false, "Show interactive picker to choose agent")
	debugCmd.Flags().Bool(debugRestoreFlag, false, "Restore selected runbook failure commit in a temporary worktree")
}

func runDebug(cmd *cobra.Command, args []string) error {
	// Get config and directories
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}
	gromitDir := resolveGromitDir(cfg)
	reportsDir := filepath.Join(gromitDir, "reports")
	plansDir := resolvePlansDir(cfg)

	// Ensure reports directory exists
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return fmt.Errorf("creating reports dir: %w", err)
	}

	// Record existing artifacts before session
	existingReports, err := getReportFiles(reportsDir)
	if err != nil {
		return fmt.Errorf("scanning reports directory: %w", err)
	}

	existingPlans, err := getPlanFiles(plansDir)
	if err != nil {
		return fmt.Errorf("scanning plans directory: %w", err)
	}

	// Load backlog to track existing items
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		return fmt.Errorf("creating backlog file: %w", err)
	}

	existingBacklogItems, err := bf.List()
	if err != nil {
		return fmt.Errorf("loading backlog: %w", err)
	}

	// Optionally show runbook picker when no description arg provided
	ttlDays := config.DefaultRunbookTTLDays
	if cfg != nil && cfg.Runbook.TTLDays > 0 {
		ttlDays = cfg.Runbook.TTLDays
	}
	selectedEntry, err := resolveRunbookEntry(gromitDir, ttlDays, args, os.Stdin)
	if err != nil {
		return fmt.Errorf("selecting runbook entry: %w", err)
	}

	mainDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining working directory: %w", err)
	}

	restoreEnabled, _ := cmd.Flags().GetBool(debugRestoreFlag)
	restoreWorktreeDir := maybeCreateDebugRestoreWorktree(restoreEnabled, gromitDir, mainDir, selectedEntry, debugGitRun, os.Stderr)
	launchDir := restoreWorktreeDir

	// Build system prompt with full project context
	systemPrompt, err := buildDebugPrompt(cfg, gromitDir, args, selectedEntry)
	if err != nil {
		return fmt.Errorf("building debug prompt: %w", err)
	}

	// Write system prompt to a temp file to avoid "argument list too long" errors
	tmpDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "debug-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(systemPrompt); err != nil {
		promptFile.Close()
		return fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	agentFlag, _ := cmd.Flags().GetString(debugAgentFlag)
	chooseAgent, _ := cmd.Flags().GetBool(debugChooseAgentFlag)

	selectedAgent, err := resolveDebugAgent(cfg, agentFlag, chooseAgent)
	if err != nil {
		return fmt.Errorf("resolving agent: %w", err)
	}

	debugWarnModelFn(cmd, selectedAgent, os.Stderr)

	if shouldOverrideDebugModel(cmd, selectedAgent) {
		binary := claudeAgentName
		var flags []string
		if cfg != nil {
			binary = cfg.Claude.Binary
			flags = cfg.Claude.Flags
		}
		flags = append(append([]string{}, flags...), "--model", debugModel)
		selectedAgent = agent.New(claudeAgentName, binary, flags, agent.FileRef, "", nil)
	}

	if err := launchDebugSession(cmd.Context(), cfg, gromitDir, selectedAgent, promptPath, launchDir); err != nil {
		return fmt.Errorf("launching agent: %w", err)
	}

	maybeCleanupDebugRestoreWorktree(restoreWorktreeDir, mainDir, os.Stdin, debugGitRun, os.Stderr)

	// Post-session artifact detection
	return detectAndReportArtifacts(reportsDir, plansDir, existingReports, existingPlans, existingBacklogItems, bf, cfg)
}

func launchDebugSession(ctx context.Context, cfg *config.Config, gromitDir string, selectedAgent agent.Agent, promptPath, launchDir string) error {
	if selectedAgent == nil {
		return fmt.Errorf("selected agent is nil")
	}
	absPromptPath, err := filepath.Abs(promptPath)
	if err != nil {
		return fmt.Errorf("resolving prompt path: %w", err)
	}

	return launchInSessionIfEnabledWithContext(ctx, cfg, gromitDir, debugSessionCommand, debugSessionLauncherFn, func(sessionDir string) error {
		effectiveDir := sessionDir
		if strings.TrimSpace(launchDir) != "" {
			effectiveDir = launchDir
		}
		return selectedAgent.LaunchInDir(absPromptPath, effectiveDir)
	}, func() error {
		return selectedAgent.LaunchInDir(absPromptPath, launchDir)
	})
}

func shouldOverrideDebugModel(cmd *cobra.Command, selectedAgent agent.Agent) bool {
	if cmd == nil || selectedAgent == nil || selectedAgent.Name() != claudeAgentName {
		return false
	}

	modelFlag := cmd.Flags().Lookup(debugModelFlag)
	if modelFlag == nil {
		return false
	}

	return cmd.Flags().Changed(debugModelFlag)
}

func maybeWarnModelFlagOnNonClaudeAgent(cmd *cobra.Command, selectedAgent agent.Agent, stderr io.Writer) {
	if cmd == nil || selectedAgent == nil {
		return
	}

	if selectedAgent.Name() == claudeAgentName {
		return
	}

	modelFlag := cmd.Flags().Lookup(debugModelFlag)
	if modelFlag == nil {
		return
	}

	if !cmd.Flags().Changed(debugModelFlag) {
		return
	}

	fmt.Fprintf(stderr, "Warning: --model flag ignored for non-Claude agent %q\n", selectedAgent.Name())
}

func resolveDebugAgent(cfg *config.Config, agentFlag string, chooseAgent bool) (agent.Agent, error) {
	return resolveCommandAgent(cfg, debugSessionCommand, agentFlag, chooseAgent)
}

func runDebugGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func maybeCreateDebugRestoreWorktree(enabled bool, gromitDir, mainDir string, entry *runbook.Entry, gitRun debugGitRunFn, stderr io.Writer) string {
	if !enabled || entry == nil {
		return ""
	}

	failureCommit := strings.TrimSpace(entry.FailureCommit)
	if failureCommit == "" {
		fmt.Fprintf(stderr, "Warning: selected runbook entry %q has no failure_commit; using context-only mode\n", entry.ID)
		return ""
	}

	worktreesDir := filepath.Join(gromitDir, debugWorktreesDir)
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "Warning: failed to create debug worktrees dir: %v; using context-only mode\n", err)
		return ""
	}

	worktreeDir := filepath.Join(worktreesDir, debugWorktreePrefix+sanitizeRunbookIDForPath(entry.ID))
	if _, err := gitRun(mainDir, gitWorktreeCmd, gitWorktreeAddCmd, gitDetachFlag, worktreeDir, failureCommit); err != nil {
		fmt.Fprintf(stderr, "Warning: failed to restore failure commit %s in worktree: %v; using context-only mode\n", failureCommit, err)
		return ""
	}

	return worktreeDir
}

func maybeCleanupDebugRestoreWorktree(worktreeDir, mainDir string, input io.Reader, gitRun debugGitRunFn, stderr io.Writer) {
	if strings.TrimSpace(worktreeDir) == "" {
		return
	}

	reader := input
	if reader == nil {
		reader = os.Stdin
	}

	if debugConfirmPromptFn(bufio.NewReader(reader), debugKeepPrompt, false) {
		return
	}

	if _, err := gitRun(mainDir, gitWorktreeCmd, gitWorktreeRemoveCmd, worktreeDir); err != nil {
		fmt.Fprintf(stderr, "Warning: failed to remove debug worktree %s: %v\n", worktreeDir, err)
	}
}

func sanitizeRunbookIDForPath(runbookID string) string {
	sanitized := strings.TrimSpace(runbookID)
	if sanitized == "" {
		return "unknown"
	}

	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	return replacer.Replace(sanitized)
}

// buildDebugPrompt constructs the system prompt for the debug session.
// If entry is non-nil, a Failure Context section is injected before the Context section.
func buildDebugPrompt(cfg *config.Config, gromitDir string, args []string, entry *runbook.Entry) (string, error) {
	// Load project context
	templatesDir := resolveTemplatesDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	claudeMDPath := resolveProjectClaudeMD(cfg)

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return "", fmt.Errorf("creating prompt renderer: %w", err)
	}

	claudeMD, err := renderer.LoadClaudeMD()
	if err != nil {
		return "", fmt.Errorf("loading CLAUDE.md: %w", err)
	}

	rules, err := renderer.LoadRules()
	if err != nil {
		return "", fmt.Errorf("loading RULES.md: %w", err)
	}

	// Load learnings
	lf := renderer.GetLearningsFile()
	var confirmedLearnings, recentLearnings string
	confirmedLearnings = formatLearnings(lf, true)
	recentLearnings = formatLearnings(lf, false)

	// Get working directory
	workDir, _ := os.Getwd()

	// Extract validation commands
	var validationCommands string
	if cfg != nil && cfg.Validation.Enabled && len(cfg.Validation.Commands) > 0 {
		var sb strings.Builder
		for _, cmd := range cfg.Validation.Commands {
			sb.WriteString(fmt.Sprintf("   %s\n", cmd))
		}
		validationCommands = strings.TrimRight(sb.String(), "\n")
	} else {
		validationCommands = "   # No validation commands configured"
	}

	// Determine reports directory
	reportsDir := filepath.Join(gromitDir, "reports")
	compatibilityDiagnostics := formatDebugCompatibilityDiagnostics(cfg)

	// Build the system prompt
	var sb strings.Builder

	// Optional: pre-seeded bug description
	if len(args) > 0 {
		sb.WriteString(fmt.Sprintf(`Bug description:

%s

`, args[0]))
	}

	// Optional: failure context from runbook entry
	if entry != nil {
		diffCmd := fmt.Sprintf("git diff %s %s", entry.StartCommit, entry.FailureCommit)
		sb.WriteString(fmt.Sprintf("## Failure Context\n\n**Bead:** %s — %s\n**Failure Category:** %s\n**Commits:** %s..%s (run `%s` to see changes)\n\n### Validation Commands\n\n%s\n\n### Failure Output\n\n%s\n\n### Build Prompt\n\n%s\n\n",
			entry.BeadID, entry.BeadTitle, entry.FailureCategory,
			entry.StartCommit, entry.FailureCommit, diffCmd,
			strings.Join(entry.ValidationCommands, "\n"),
			entry.FailureOutput,
			entry.Prompt))
	}

	// Context section
	sb.WriteString(fmt.Sprintf(`## Context

Working directory: %s
Reports directory: %s
%s

### Validation Commands

When applying direct fixes (outcome 1), run these commands:

%s

	`, workDir, reportsDir, compatibilityDiagnostics, validationCommands))

	// Project context
	if claudeMD != "" {
		sb.WriteString(fmt.Sprintf(`### Project Context

%s

`, claudeMD))
	}

	if rules != "" {
		sb.WriteString(fmt.Sprintf(`### Rules (Non-Negotiable)

%s

`, rules))
	}

	// Learnings
	sb.WriteString(fmt.Sprintf(`### Learnings

#### Confirmed Patterns

%s

#### Recent Learnings

%s

`, confirmedLearnings, recentLearnings))

	// Embedded skill instructions
	sb.WriteString(fmt.Sprintf(`## Instructions

%s`, skills.DebugSkill))

	return sb.String(), nil
}

func formatDebugCompatibilityDiagnostics(cfg *config.Config) string {
	resolved := config.Config{}.ResolveCompatibilityContext()
	if cfg != nil {
		resolved = cfg.ResolveCompatibilityContext()
	}

	diagnostics := fmt.Sprintf("Compatibility:\n  Profile:  %s (source: %s)\n  Backend:  %s (source: %s)\n  Adapter:  %s (source: %s)",
		resolved.Profile.Value, resolved.Profile.Source,
		resolved.TrackerBackend.Value, resolved.TrackerBackend.Source,
		resolved.MethodologyAdapter.Value, resolved.MethodologyAdapter.Source,
	)
	markers := config.CompatibilityDeprecationMarkers(resolved)
	if len(markers) > 0 {
		diagnostics += fmt.Sprintf("\n  Deprecation markers: %s\n  Strict default cutoff: %s", strings.Join(markers, ", "), config.CompatibilityStrictDefaultCutoverDate)
	}
	return diagnostics
}

// getMDFiles returns a list of .md files in the given directory
func getMDFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	mdFiles := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdFiles = append(mdFiles, filepath.Join(dir, entry.Name()))
		}
	}

	return mdFiles, nil
}

// getReportFiles returns a list of .md files in the reports directory
func getReportFiles(reportsDir string) ([]string, error) {
	return getMDFiles(reportsDir)
}

// getPlanFiles returns a list of .md files in the plans directory
func getPlanFiles(plansDir string) ([]string, error) {
	return getMDFiles(plansDir)
}

// getNewBacklogItems returns backlog items that are not in the existing list
func getNewBacklogItems(existing []*backlog.Idea, bf *backlog.File) ([]*backlog.Idea, error) {
	current, err := bf.List()
	if err != nil {
		return nil, err
	}

	// Build set of existing IDs
	existingIDs := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		existingIDs[item.ID] = struct{}{}
	}

	// Find new items
	newItems := make([]*backlog.Idea, 0, len(current))
	for _, item := range current {
		if _, ok := existingIDs[item.ID]; !ok {
			newItems = append(newItems, item)
		}
	}

	return newItems, nil
}

func newArtifactPaths(existing, current []string) []string {
	if len(current) == 0 {
		return []string{}
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, path := range existing {
		existingSet[path] = struct{}{}
	}

	created := make([]string, 0, len(current))
	for _, path := range current {
		if _, ok := existingSet[path]; !ok {
			created = append(created, path)
		}
	}

	return created
}

// detectAndReportArtifacts scans for new reports, plans, and backlog items,
// then displays a summary and offers chaining options
func detectAndReportArtifacts(reportsDir, plansDir string, existingReports, existingPlans []string, existingBacklogItems []*backlog.Idea, bf *backlog.File, cfg *config.Config) error {
	// Scan for new artifacts
	newReports, err := getReportFiles(reportsDir)
	if err != nil {
		return fmt.Errorf("scanning reports directory: %w", err)
	}

	newPlans, err := getPlanFiles(plansDir)
	if err != nil {
		return fmt.Errorf("scanning plans directory: %w", err)
	}

	newBacklogItems, err := getNewBacklogItems(existingBacklogItems, bf)
	if err != nil {
		return fmt.Errorf("scanning backlog: %w", err)
	}

	// Find newly created artifacts
	createdReports := newArtifactPaths(existingReports, newReports)
	createdPlans := newArtifactPaths(existingPlans, newPlans)

	// Display summary
	hasArtifacts := len(createdReports) > 0 || len(createdPlans) > 0 || len(newBacklogItems) > 0

	if hasArtifacts {
		fmt.Println()
	}

	if len(createdReports) > 0 {
		fmt.Println("Investigation reports created:")
		for _, report := range createdReports {
			fmt.Printf("  - %s\n", report)
		}
		fmt.Println()
	}

	if len(newBacklogItems) > 0 {
		fmt.Println("Backlog items created:")
		for _, item := range newBacklogItems {
			fmt.Printf("  - %s: %s\n", item.ID, item.Text)
		}
		fmt.Println()
	}

	// Offer chaining if plans were created
	if len(createdPlans) > 0 {
		fmt.Println("Plan files created:")
		for _, plan := range createdPlans {
			fmt.Printf("  - %s\n", plan)
		}
		fmt.Println()

		// Extract plan names for chaining
		planNames := make([]string, 0, len(createdPlans))
		for _, planPath := range createdPlans {
			planName := strings.TrimSuffix(filepath.Base(planPath), ".md")
			planNames = append(planNames, planName)
		}

		reader := bufio.NewReader(os.Stdin)
		chainAfterRefine(planNames, plansDir, func(prompt string, defaultYes bool) bool {
			return confirmPrompt(reader, prompt, defaultYes)
		}, execGromit)
	}

	return nil
}

// resolveRunbookEntry returns a runbook entry to inject as failure context.
// If args are non-empty (description provided), returns nil (picker skipped).
// Otherwise calls runbook.List and shows the picker.
func resolveRunbookEntry(gromitDir string, ttlDays int, args []string, r io.Reader) (*runbook.Entry, error) {
	if len(args) > 0 {
		return nil, nil
	}
	entries, err := runbook.List(gromitDir, ttlDays)
	if err != nil {
		return nil, fmt.Errorf("listing runbook entries: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return pickRunbookEntry(entries, r)
}

// pickRunbookEntry displays a numbered menu of runbook entries and returns the selected one.
// Returns nil if the user selects 0 or enters an invalid selection.
func pickRunbookEntry(entries []runbook.Entry, r io.Reader) (*runbook.Entry, error) {
	fmt.Println("Recent failures (select to load context, 0 to skip):")
	for i, e := range entries {
		ago := time.Since(e.Timestamp).Round(time.Minute)
		fmt.Printf("  %d. [%s ago] %s — %s (%s)\n", i+1, ago, e.BeadID, e.BeadTitle, e.FailureCategory)
	}
	fmt.Print("Selection: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, nil
	}
	line := strings.TrimSpace(scanner.Text())
	n, err := strconv.Atoi(line)
	if err != nil || n <= 0 || n > len(entries) {
		return nil, nil
	}
	result := entries[n-1]
	return &result, nil
}
