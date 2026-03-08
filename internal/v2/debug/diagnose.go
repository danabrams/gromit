package debug

import (
	"strings"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/pipeline"
)

// RootCause identifies the likely reason a run failed.
type RootCause string

const (
	RootCauseBadBuildOutput       RootCause = "bad_build_output"
	RootCauseFlakyTest            RootCause = "flaky_test"
	RootCauseUnclearBead          RootCause = "unclear_bead_description"
	RootCauseBadDecomposition     RootCause = "incorrect_decomposition"
	defaultFailureStage                     = "unknown"
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
}

// Diagnose finds the failure point and classifies a root cause.
func Diagnose(input Input) Diagnosis {
	diag := Diagnosis{
		Stage:     defaultFailureStage,
		RootCause: RootCauseBadBuildOutput,
	}

	if evt := findFailureEvent(input.Events); evt != nil {
		diag.FailureEvent = evt
		if stage := strings.TrimSpace(valueAsString(evt["stage_name"])); stage != "" {
			diag.Stage = stage
		}
		diag.RootCause = classifyRootCause(evt, diag.Stage)
	}

	if commit := findFailureCommit(input.LogEntries); commit != "" {
		diag.FailureCommit = commit
	}

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
	if strings.Contains(text, "flaky") || strings.Contains(text, "timeout") {
		return RootCauseFlakyTest
	}
	if strings.Contains(text, "decompose") || strings.Contains(text, "decomposition") {
		return RootCauseBadDecomposition
	}
	if strings.Contains(text, "unclear") || strings.Contains(text, "ambiguous") {
		return RootCauseUnclearBead
	}
	if stage == "build" {
		return RootCauseBadBuildOutput
	}
	return RootCauseBadBuildOutput
}

func findFailureCommit(entries []adapter.LogEntry) string {
	for _, entry := range entries {
		info, ok := pipeline.ParseCommitMessage(entry.Message)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(info.Decision), "Fail") {
			return entry.Hash
		}
	}
	return ""
}

func valueAsString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
