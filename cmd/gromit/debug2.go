package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	llmadapter "github.com/danabrams/gromit/internal/v2/adapter/llm"
	debugpkg "github.com/danabrams/gromit/internal/v2/debug"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	"github.com/danabrams/gromit/internal/v2/routing"
	"github.com/spf13/cobra"
)

type debug2LLMFixResponse struct {
	CodePatch              string `json:"code_patch"`
	LearningsEntry         string `json:"learnings_entry"`
	SystemicRecommendation string `json:"systemic_recommendation"`
}

// debug2BranchWorktreeFn is injectable for tests. It attempts to find and resolve
// a worktree from the branch gromit/spec/<specName> when the directory is missing.
// t.Cleanup must restore the original value in tests that override this.
var debug2BranchWorktreeFn = func(gromitDir, specName string) (string, error) {
	repoRoot := filepath.Dir(gromitDir)
	targetBranch := "gromit/spec/" + specName

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	var currentWorktree string
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
			continue
		}
		if strings.HasPrefix(line, "branch refs/heads/") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if branch == targetBranch && currentWorktree != "" {
				return normalizeDebug2WorktreePath(currentWorktree), nil
			}
		}
	}

	return "", fmt.Errorf("no preserved worktree found for branch %q", targetBranch)
}

var debug2Cmd = &cobra.Command{
	Use:   "debug2 <spec-name>",
	Short: "Diagnose and fix a failed v2 spec execution",
	Args:  cobra.ExactArgs(1),
	RunE:  debug2RunE,
}

const debug2EventTailCount = 2
const debug2Phase = "debug"
const debug2DefaultInvokeTimeout = 15 * time.Minute

var debug2ImplFn = debug2Impl
var debug2InvokeLLMFn = invokeDebug2LLM
var debug2ApplyPatchFn = applyDebug2Patch
var debug2CheckoutFailureFn = checkoutDebug2FailureCommit
var debug2RunValidationFn = runDebug2ValidationCommand
var debug2Stderr io.Writer = os.Stderr
var debug2CommitHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

func init() {
	rootCmd.AddCommand(debug2Cmd)
}

// resolveDebug2Worktree returns the path to the preserved spec worktree. It first
// tries to find the worktree at .gromit/spec-worktrees/<specName>. If that doesn't
// exist, it falls back to trying to find the branch gromit/spec/<specName>.
func resolveDebug2Worktree(gromitDir, specName string) (string, error) {
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)
	if _, err := os.Stat(wtPath); err == nil {
		return wtPath, nil
	}

	// Worktree directory not found; try to find the branch instead
	return debug2BranchWorktreeFn(gromitDir, specName)
}

// readDebug2EventLog reads and parses the JSONL event log from a spec worktree.
// Each line is decoded as a map[string]interface{}.
func readDebug2EventLog(wtPath string) ([]map[string]interface{}, error) {
	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("opening event log: %w", err)
	}
	defer f.Close()

	var events []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parsing event line: %w", err)
		}
		events = append(events, entry)
	}
	return events, scanner.Err()
}

// buildDebug2Prompt assembles a diagnostic prompt from the spec name, worktree path,
// events, and commit history. commits is a slice of [hash, message] pairs.
func buildDebug2Prompt(specName, wtPath string, events []map[string]interface{}, commits [][2]string, failureDiff string, validationCommands []string, diagnosis debugpkg.Diagnosis) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Debug Session: %s\n\n", specName))
	sb.WriteString(fmt.Sprintf("Worktree: %s\n\n", wtPath))

	sb.WriteString("### Commit History\n\n")
	for _, c := range commits {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c[0], c[1]))
	}
	sb.WriteString("\n")

	sb.WriteString("### Event Log Tail\n\n")
	for _, e := range events {
		data, _ := json.Marshal(e)
		sb.WriteString(fmt.Sprintf("  %s\n", string(data)))
	}
	sb.WriteString("\n")

	failEvent := diagnosis.FailureEvent
	if failEvent == nil {
		failEvent = findFailureEvent(events)
	}
	if failEvent != nil {
		sb.WriteString("### Failure Point\n\n")
		data, _ := json.Marshal(failEvent)
		sb.WriteString(fmt.Sprintf("  %s\n\n", string(data)))
	}
	if diagnosis.Stage != "" && diagnosis.Stage != "unknown" {
		sb.WriteString("### Failure Stage\n\n")
		sb.WriteString(fmt.Sprintf("  %s\n\n", diagnosis.Stage))
	}
	if diagnosis.RootCause != "" {
		sb.WriteString("### Root Cause\n\n")
		sb.WriteString(fmt.Sprintf("  %s\n\n", diagnosis.RootCause))
	}
	if diagnosis.FailureCommit != "" {
		sb.WriteString("### Failure Commit\n\n")
		sb.WriteString(fmt.Sprintf("  %s\n\n", diagnosis.FailureCommit))
	}

	if strings.TrimSpace(failureDiff) != "" {
		sb.WriteString("### Failure Diff\n\n")
		sb.WriteString("```diff\n")
		sb.WriteString(failureDiff)
		if !strings.HasSuffix(failureDiff, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
	}

	sb.WriteString("### Validation Commands\n\n")
	if len(validationCommands) == 0 {
		sb.WriteString("  # No validation commands configured\n\n")
	} else {
		for _, cmd := range validationCommands {
			sb.WriteString(fmt.Sprintf("  %s\n", cmd))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Task\n\nDiagnose the failure above and produce a fix. Return JSON only with:\n")
	sb.WriteString(`{"code_patch":"<unified diff patch or empty>","learnings_entry":"<entry or empty>","systemic_recommendation":"<text or empty>"}` + "\n")
	return sb.String()
}

// findFailureEvent returns the first event with decision "Fail", or nil if none found.
func findFailureEvent(events []map[string]interface{}) map[string]interface{} {
	for _, e := range events {
		if e["decision"] == "Fail" {
			return e
		}
	}
	return nil
}

func tailDebug2Events(events []map[string]interface{}, n int) []map[string]interface{} {
	if n <= 0 || len(events) == 0 {
		return nil
	}
	if len(events) <= n {
		return events
	}
	return events[len(events)-n:]
}

func selectDebug2FailureCommit(entries []adapter.LogEntry) (adapter.LogEntry, pipeline.CommitInfo, bool) {
	for _, entry := range entries {
		info, ok := pipeline.ParseCommitMessage(entry.Message)
		if !ok {
			continue
		}
		if info.Decision == "Fail" {
			return entry, info, true
		}
	}
	return adapter.LogEntry{}, pipeline.CommitInfo{}, false
}

func normalizeDebug2WorktreePath(path string) string {
	normalized := filepath.Clean(path)
	if runtime.GOOS == "darwin" && strings.HasPrefix(normalized, "/private/var/") {
		return strings.TrimPrefix(normalized, "/private")
	}
	return normalized
}

func debug2ValidationCommands(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	commands := cfg.Validation.FastCommandsOrDefault()
	if len(commands) == 0 {
		commands = cfg.EffectiveValidationCommands()
	}
	return append([]string(nil), commands...)
}

func selectDebug2Provider(cfg *config.Config) (llmtypes.LLMProvider, string, error) {
	if cfg != nil {
		router, phaseModels := buildRouter(cfg)
		if router != nil {
			tier := routing.TierForPhase(debug2Phase, phaseModels, routing.TierMedium)
			provider, model, _, err := router.Select(debug2Phase, tier)
			if err != nil {
				return nil, "", fmt.Errorf("selecting routed provider: %w", err)
			}
			return provider, model, nil
		}

		binary := strings.TrimSpace(cfg.Claude.Binary)
		if binary == "" {
			binary = "claude"
		}
		flags := append([]string(nil), cfg.Claude.Flags...)
		timeout := debug2DefaultInvokeTimeout
		if cfg.Claude.Timeout > 0 {
			timeout = time.Duration(cfg.Claude.Timeout) * time.Second
		}
		model := strings.TrimSpace(cfg.Models.P1)
		if model == "" {
			model = config.ModelSonnet
		}
		return llmadapter.NewClaudeAdapter(binary, flags, timeout), model, nil
	}

	return llmadapter.NewClaudeAdapter("claude", nil, debug2DefaultInvokeTimeout), config.ModelSonnet, nil
}

func invokeDebug2LLM(ctx context.Context, prompt, dir string, cfg *config.Config) (string, error) {
	provider, model, err := selectDebug2Provider(cfg)
	if err != nil {
		return "", err
	}
	resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{
		Prompt: prompt,
		Model:  model,
		Dir:    dir,
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("provider returned nil response")
	}
	if !resp.Success {
		detail := strings.TrimSpace(resp.Output)
		if detail == "" {
			detail = "no detail available"
		}
		return "", fmt.Errorf("provider reported unsuccessful result: %s", detail)
	}
	return resp.Output, nil
}

func parseDebug2LLMResponse(output string) (*debug2LLMFixResponse, error) {
	text := strings.TrimSpace(output)
	if text == "" {
		return nil, fmt.Errorf("llm output is empty")
	}
	var response debug2LLMFixResponse
	if err := json.Unmarshal([]byte(text), &response); err != nil {
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start == -1 || end == -1 || end <= start {
			return nil, fmt.Errorf("extracting llm response json: no JSON object found")
		}
		if err := json.Unmarshal([]byte(text[start:end+1]), &response); err != nil {
			return nil, fmt.Errorf("extracting llm response json: %w", err)
		}
	}
	response.LearningsEntry = strings.TrimSpace(response.LearningsEntry)
	response.SystemicRecommendation = strings.TrimSpace(response.SystemicRecommendation)
	return &response, nil
}

func applyDebug2Patch(ctx context.Context, wtPath, patch string) error {
	if strings.TrimSpace(patch) == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn", "-")
	cmd.Dir = wtPath
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("applying debug patch: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func checkoutDebug2FailureCommit(ctx context.Context, wtPath, commit string) error {
	trimmed := strings.TrimSpace(commit)
	if trimmed == "" {
		return nil
	}
	if !debug2CommitHashPattern.MatchString(trimmed) {
		return fmt.Errorf("invalid failure commit %q", commit)
	}

	cmd := exec.CommandContext(ctx, "git", "checkout", trimmed)
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("checking out failure commit %s: %w\n%s", trimmed, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runDebug2ValidationCommand(ctx context.Context, wtPath, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validation command %q failed: %w\n%s", command, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func appendDebug2LearningsEntry(wtPath, entry string) error {
	trimmedEntry := strings.TrimSpace(entry)
	if trimmedEntry == "" {
		return nil
	}

	learningsPath := filepath.Join(wtPath, "LEARNINGS.md")
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(learningsPath, []byte(trimmedEntry+"\n"), 0o644)
		}
		return fmt.Errorf("reading LEARNINGS.md: %w", err)
	}

	text := string(content)
	insertBlock := trimmedEntry + "\n\n"
	marker := "\n## Provisional Learnings"
	if idx := strings.Index(text, marker); idx >= 0 {
		prefix := strings.TrimRight(text[:idx], "\n")
		suffix := strings.TrimLeft(text[idx:], "\n")
		updated := prefix + "\n\n" + insertBlock + suffix
		return os.WriteFile(learningsPath, []byte(updated), 0o644)
	}

	updated := strings.TrimRight(text, "\n")
	if updated != "" {
		updated += "\n\n"
	}
	updated += trimmedEntry + "\n"
	return os.WriteFile(learningsPath, []byte(updated), 0o644)
}

// debug2Impl contains the testable core of the debug2 command.
func debug2Impl(ctx context.Context, specName, gromitDir string, cfg *config.Config) error {
	if ctx == nil {
		ctx = context.Background()
	}

	wtPath, err := resolveDebug2Worktree(gromitDir, specName)
	if err != nil {
		return err
	}

	events, err := readDebug2EventLog(wtPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading event log: %w", err)
	}

	gitAdapter := gitadapter.NewExecGitAdapter(".", gromitDir)
	logEntries, err := gitAdapter.Log(ctx, wtPath, 100)
	if err != nil {
		logEntries = nil // non-fatal: proceed without commit history
	}
	diagnosis := debugpkg.Diagnose(debugpkg.Input{
		Events:     events,
		LogEntries: logEntries,
	})

	commits := make([][2]string, 0, len(logEntries))
	for _, e := range logEntries {
		hash := e.Hash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		commits = append(commits, [2]string{hash, e.Message})
	}

	failureDiff := ""
	if diagnosis.FailureCommit != "" {
		diff, showErr := gitAdapter.Show(ctx, wtPath, diagnosis.FailureCommit)
		if showErr == nil {
			failureDiff = diff
		}
	} else if failureCommit, _, ok := selectDebug2FailureCommit(logEntries); ok {
		diff, showErr := gitAdapter.Show(ctx, wtPath, failureCommit.Hash)
		if showErr == nil {
			failureDiff = diff
		}
	}

	validationCommands := debug2ValidationCommands(cfg)
	prompt := buildDebug2Prompt(specName, wtPath, tailDebug2Events(events, debug2EventTailCount), commits, failureDiff, validationCommands, diagnosis)
	responseText, err := debug2InvokeLLMFn(ctx, prompt, wtPath, cfg)
	if err != nil {
		return fmt.Errorf("invoking debug llm: %w", err)
	}
	response, err := parseDebug2LLMResponse(responseText)
	if err != nil {
		return err
	}
	if strings.TrimSpace(response.CodePatch) != "" && diagnosis.FailureCommit != "" {
		if err := debug2CheckoutFailureFn(ctx, wtPath, diagnosis.FailureCommit); err != nil {
			return err
		}
	}
	if err := debug2ApplyPatchFn(ctx, wtPath, response.CodePatch); err != nil {
		return err
	}
	for _, command := range validationCommands {
		if err := debug2RunValidationFn(ctx, wtPath, command); err != nil {
			return err
		}
	}
	if err := appendDebug2LearningsEntry(wtPath, response.LearningsEntry); err != nil {
		return err
	}
	if response.SystemicRecommendation != "" {
		fmt.Fprintf(debug2Stderr, "Systemic recommendation (not auto-applied):\n%s\n", response.SystemicRecommendation)
	}
	return nil
}

func debug2RunE(cmd *cobra.Command, args []string) error {
	specName := args[0]
	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}
	gromitDir := resolveGromitDir(cfg)
	return debug2ImplFn(cmd.Context(), specName, gromitDir, cfg)
}
