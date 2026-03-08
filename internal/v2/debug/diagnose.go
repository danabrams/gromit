package debug

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/pipeline"
)

// RootCause identifies the likely reason a run failed.
type RootCause string

const (
	RootCauseBadBuildOutput    RootCause = "bad_build_output"
	RootCauseFlakyTest         RootCause = "flaky_test"
	RootCauseUnclearBead       RootCause = "unclear_bead_description"
	RootCauseBadDecomposition  RootCause = "incorrect_decomposition"
	defaultFailureStage                  = "unknown"
	defaultDiagnosisLogEntries           = 100
)

// Input is the diagnostic context for failure analysis.
type Input struct {
	Events     []map[string]interface{}
	LogEntries []adapter.LogEntry
}

// Diagnosis captures failure point and root cause for a failed run.
type Diagnosis struct {
	FailureEvent  map[string]interface{}
	Stage         string
	FailureCommit string
	RootCause     RootCause
	StageTrace    StageTrace
	Summary       string
}

// WorktreeDiagnosisResult captures the diagnosis plus the source artifacts.
type WorktreeDiagnosisResult struct {
	WorktreePath string
	Events       []map[string]interface{}
	LogEntries   []adapter.LogEntry
	Diagnosis    Diagnosis
}

// ResolveSpecWorktree locates a preserved worktree path for the given spec.
// It first looks for a cached worktree under .gromit/spec-worktrees/<spec>.
// If that path does not exist, it falls back to the gromit/spec/<spec> branch.
func ResolveSpecWorktree(gromitDir, specName string) (string, error) {
	worktreePath := filepath.Join(gromitDir, "spec-worktrees", specName)
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		return worktreePath, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("checking preserved worktree path %q: %w", worktreePath, err)
	}

	branchPath, err := FindPreservedWorktreeBranch(gromitDir, specName)
	if err != nil {
		if errors.Is(err, ErrPreservedWorktreeBranchNotFound) {
			return "", fmt.Errorf("no preserved worktree or branch found for spec %q", specName)
		}
		return "", err
	}
	return branchPath, nil
}

// ReadEventLog parses the events.jsonl log for the provided worktree.
func ReadEventLog(worktree string) ([]map[string]interface{}, error) {
	return readEventLog(worktree)
}

// DiagnoseSpec reads the preserved worktree for a spec and derives the diagnosis.
func DiagnoseSpec(ctx context.Context, gromitDir, specName string, gitAdapter adapter.GitAdapter, logEntriesLimit int) (*WorktreeDiagnosisResult, error) {
	if gitAdapter == nil {
		return nil, fmt.Errorf("git adapter is required for diagnosis")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	worktreePath, err := ResolveSpecWorktree(gromitDir, specName)
	if err != nil {
		return nil, err
	}

	events, err := readEventLog(worktreePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading event log: %w", err)
	}

	limit := logEntriesLimit
	if limit <= 0 {
		limit = defaultDiagnosisLogEntries
	}
	logEntries, logErr := gitAdapter.Log(ctx, worktreePath, limit)
	if logErr != nil {
		logEntries = nil
	}

	diagnosis := Diagnose(Input{
		Events:     events,
		LogEntries: logEntries,
	})

	return &WorktreeDiagnosisResult{
		WorktreePath: worktreePath,
		Events:       events,
		LogEntries:   logEntries,
		Diagnosis:    diagnosis,
	}, nil
}

// ValidationTrace summarizes validation results for a failed stage.
type ValidationTrace struct {
	Commands      []string
	FailedCommand string
	Details       string
	Succeeded     bool
}

// StageTrace captures stage-level diagnostics from events and commits.
type StageTrace struct {
	StageName      string
	BeadID         string
	Iteration      int
	FailureMessage string
	FailureEvent   map[string]interface{}
	Events         []map[string]interface{}
	Validation     *ValidationTrace
	CommitHash     string
	CommitDecision string
}

// Diagnose finds the failure point and classifies a root cause.
func Diagnose(input Input) Diagnosis {
	diag := Diagnosis{
		Stage:     defaultFailureStage,
		RootCause: RootCauseBadBuildOutput,
	}
	trace := StageTrace{}
	failureEventHasStage := false

	if evt := findFailureEvent(input.Events); evt != nil {
		diag.FailureEvent = evt
		trace.FailureEvent = evt
		trace.StageName = strings.TrimSpace(valueAsString(evt["stage_name"]))
		trace.BeadID = strings.TrimSpace(valueAsString(evt["bead_id"]))
		trace.Iteration = valueAsInt(evt["iteration"])
		trace.FailureMessage = valueAsString(evt["error"])
		if trace.StageName != "" {
			diag.Stage = trace.StageName
			failureEventHasStage = true
		}
		diag.RootCause = classifyRootCause(evt, diag.Stage)
	}

	if commit, info, ok := findFailureCommit(input.LogEntries); ok {
		diag.FailureCommit = commit
		trace.CommitHash = commit
		trace.CommitDecision = strings.TrimSpace(info.Decision)
		if trace.StageName == "" {
			trace.StageName = strings.TrimSpace(info.StageName)
		}
		if trace.BeadID == "" {
			trace.BeadID = strings.TrimSpace(info.BeadID)
		}
		if trace.Iteration == 0 {
			trace.Iteration = info.Iteration
		}
		if diag.Stage == defaultFailureStage && trace.StageName != "" {
			diag.Stage = trace.StageName
			if diag.FailureEvent == nil || !failureEventHasStage {
				diag.RootCause = classifyRootCauseFromStage(trace.StageName)
			}
		}
	}

	if trace.StageName == "" && diag.Stage != defaultFailureStage {
		trace.StageName = diag.Stage
	}

	trace.Events = gatherStageEvents(input.Events, trace.StageName, trace.BeadID, trace.Iteration)
	trace.Validation = findValidationTrace(trace.Events)

	diag.StageTrace = trace
	diag.RootCause = refineRootCauseFromStage(trace, diag.RootCause)
	diag.Summary = buildHumanReadableSummary(trace, diag.RootCause)

	return diag
}

func findFailureEvent(events []map[string]interface{}) map[string]interface{} {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if strings.EqualFold(valueAsString(event["decision"]), "Fail") {
			return event
		}
		if strings.EqualFold(valueAsString(event["type"]), "stage.failed") {
			return event
		}
	}
	return nil
}

func classifyRootCause(event map[string]interface{}, stage string) RootCause {
	text := strings.ToLower(strings.TrimSpace(valueAsString(event["error"])))
	if strings.Contains(text, "flaky") ||
		strings.Contains(text, "timeout") ||
		(strings.EqualFold(stage, "validate") && strings.Contains(text, "retry")) ||
		(strings.EqualFold(stage, "validate") && strings.Contains(text, "transient")) {
		return RootCauseFlakyTest
	}
	if strings.Contains(text, "decompose") ||
		strings.Contains(text, "decomposition") ||
		(strings.EqualFold(stage, "decompose") && strings.Contains(text, "split")) ||
		(strings.EqualFold(stage, "decompose") && strings.Contains(text, "broad")) {
		return RootCauseBadDecomposition
	}
	if strings.Contains(text, "unclear") ||
		strings.Contains(text, "ambiguous") ||
		strings.Contains(text, "bead description") ||
		strings.Contains(text, "acceptance criteria") ||
		strings.Contains(text, "expected outputs") {
		return RootCauseUnclearBead
	}
	if stage == "build" {
		return RootCauseBadBuildOutput
	}
	return RootCauseBadBuildOutput
}

func findFailureCommit(entries []adapter.LogEntry) (string, pipeline.CommitInfo, bool) {
	for _, entry := range entries {
		info, ok := pipeline.ParseCommitMessage(entry.Message)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(info.Decision), "Fail") {
			return entry.Hash, info, true
		}
	}
	return "", pipeline.CommitInfo{}, false
}

func gatherStageEvents(events []map[string]interface{}, stageName, beadID string, iteration int) []map[string]interface{} {
	if len(events) == 0 {
		return nil
	}
	if stageName == "" && beadID == "" && iteration == 0 {
		return nil
	}

	var matches []map[string]interface{}
	for _, evt := range events {
		if stageName != "" && !strings.EqualFold(stageName, valueAsString(evt["stage_name"])) {
			continue
		}
		if beadID != "" && !strings.EqualFold(beadID, valueAsString(evt["bead_id"])) {
			continue
		}
		if iteration > 0 && valueAsInt(evt["iteration"]) != iteration {
			continue
		}
		matches = append(matches, evt)
	}
	return matches
}

func findValidationTrace(events []map[string]interface{}) *ValidationTrace {
	for _, evt := range events {
		if !strings.EqualFold(valueAsString(evt["type"]), "validation") {
			continue
		}
		return &ValidationTrace{
			Commands:      valueAsStringSlice(evt["commands"]),
			FailedCommand: valueAsString(evt["failed_command"]),
			Details:       valueAsString(evt["details"]),
			Succeeded:     valueAsBool(evt["succeeded"]),
		}
	}
	return nil
}

func valueAsInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i
		}
	}
	return 0
}

func valueAsStringSlice(v interface{}) []string {
	switch list := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(list))
		for _, item := range list {
			result = append(result, valueAsString(item))
		}
		return result
	case []string:
		out := make([]string, len(list))
		copy(out, list)
		return out
	}
	return nil
}

func valueAsBool(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		s := strings.ToLower(strings.TrimSpace(b))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return b != 0
	case int:
		return b != 0
	}
	return false
}

func valueAsString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func classifyRootCauseFromStage(stage string) RootCause {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "validate":
		return RootCauseFlakyTest
	case "decompose":
		return RootCauseBadDecomposition
	default:
		return RootCauseBadBuildOutput
	}
}

func refineRootCauseFromStage(trace StageTrace, current RootCause) RootCause {
	stage := strings.ToLower(strings.TrimSpace(trace.StageName))
	if stage == "" {
		return current
	}
	if current == RootCauseUnclearBead {
		return current
	}
	switch stage {
	case "validate", "validation":
		return RootCauseFlakyTest
	case "decompose":
		return RootCauseBadDecomposition
	}
	return current
}

func buildHumanReadableSummary(trace StageTrace, rootCause RootCause) string {
	stageName := strings.TrimSpace(trace.StageName)
	beadID := strings.TrimSpace(trace.BeadID)
	var sb strings.Builder
	if stageName != "" {
		sb.WriteString("Stage ")
		sb.WriteString(stageName)
		if beadID != "" {
			sb.WriteString(" (bead ")
			sb.WriteString(beadID)
			sb.WriteString(")")
		}
	} else if beadID != "" {
		sb.WriteString("Bead ")
		sb.WriteString(beadID)
	} else {
		sb.WriteString("Stage unknown")
	}
	if trace.Iteration > 0 {
		sb.WriteString(fmt.Sprintf(" iteration %d", trace.Iteration))
	}
	failureMsg := strings.TrimSpace(trace.FailureMessage)
	if failureMsg == "" && trace.FailureEvent != nil {
		failureMsg = strings.TrimSpace(valueAsString(trace.FailureEvent["error"]))
	}
	if failureMsg != "" {
		sb.WriteString(" failed: ")
		sb.WriteString(failureMsg)
	} else {
		sb.WriteString(" failed")
	}
	sb.WriteString(". Root cause: ")
	sb.WriteString(describeRootCause(rootCause))
	sb.WriteString(".")
	if trace.Validation != nil {
		parts := make([]string, 0, 2)
		if cmd := strings.TrimSpace(trace.Validation.FailedCommand); cmd != "" {
			parts = append(parts, fmt.Sprintf("command=%s", cmd))
		}
		if details := strings.TrimSpace(trace.Validation.Details); details != "" {
			parts = append(parts, fmt.Sprintf("details=%s", details))
		}
		if len(parts) > 0 {
			sb.WriteString(" Validation ")
			sb.WriteString(strings.Join(parts, " "))
			sb.WriteString(".")
		}
	}
	return strings.TrimSpace(sb.String())
}

func describeRootCause(rootCause RootCause) string {
	switch rootCause {
	case RootCauseBadBuildOutput:
		return "bad build output"
	case RootCauseFlakyTest:
		return "flaky or transient validation failure"
	case RootCauseUnclearBead:
		return "unclear bead description"
	case RootCauseBadDecomposition:
		return "incorrect decomposition"
	default:
		return "unknown failure"
	}
}

func readEventLog(worktree string) ([]map[string]interface{}, error) {
	eventsPath := filepath.Join(worktree, ".gromit", "v2", "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parsing event line: %w", err)
		}
		events = append(events, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
