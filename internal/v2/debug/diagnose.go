package debug

import (
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/pipeline"
)

// RootCause identifies the likely reason a run failed.
type RootCause string

const (
	RootCauseBadBuildOutput   RootCause = "bad_build_output"
	RootCauseFlakyTest        RootCause = "flaky_test"
	RootCauseUnclearBead      RootCause = "unclear_bead_description"
	RootCauseBadDecomposition RootCause = "incorrect_decomposition"
	defaultFailureStage                 = "unknown"
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

	if commit, stage := findFailureCommit(input.LogEntries); commit != "" {
		diag.FailureCommit = commit
		if diag.Stage == defaultFailureStage && stage != "" {
			diag.Stage = stage
			if diag.FailureEvent == nil || !failureEventHasStage {
				diag.RootCause = classifyRootCauseFromStage(stage)
			}
		}
	}

	if trace.StageName == "" && diag.Stage != defaultFailureStage {
		trace.StageName = diag.Stage
	}

	trace.Events = gatherStageEvents(input.Events, trace.StageName, trace.BeadID, trace.Iteration)
	trace.Validation = findValidationTrace(trace.Events)

	diag.StageTrace = trace

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

func findFailureCommit(entries []adapter.LogEntry) (string, string) {
	for _, entry := range entries {
		info, ok := pipeline.ParseCommitMessage(entry.Message)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(info.Decision), "Fail") {
			return entry.Hash, strings.TrimSpace(info.StageName)
		}
	}
	return "", ""
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
