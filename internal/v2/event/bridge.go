package event

import (
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

func legacyEventsFromTyped(evt TypedEvent) []events.Event {
	if evt == nil {
		return nil
	}

	switch e := evt.(type) {
	case *SpecStartedEvent:
		return []events.Event{&events.SpecStartedEvent{
			SpecID:   e.SpecID,
			Worktree: e.Worktree,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *SpecCompletedEvent:
		return []events.Event{&events.SpecCompletedEvent{
			SpecID:        e.SpecID,
			Worktree:      e.Worktree,
			Success:       e.Success,
			FailureReason: e.FailureReason,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *SpecFailedEvent:
		return []events.Event{&events.SpecFailedEvent{
			SpecID:        e.SpecID,
			Worktree:      e.Worktree,
			FailureReason: e.FailureReason,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *BeadStartedEvent:
		return []events.Event{&events.IterationStartEvent{
			Iteration: e.Iteration,
			BeadID:    e.BeadID,
			BeadTitle: e.BeadTitle,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *BeadCompletedEvent:
		return convertBeadCompleted(e)
	case *StageStartedEvent:
		return []events.Event{logEvent("info", fmt.Sprintf("stage %s started for bead %s (iteration %d)", e.StageName, e.BeadID, e.Iteration), e.Timestamp)}
	case *StageCompletedEvent:
		status := "completed"
		if !e.Success {
			status = "failed"
		}
		return []events.Event{logEvent("info", fmt.Sprintf("stage %s %s for bead %s (iteration %d, duration %v)", e.StageName, status, e.BeadID, e.Iteration, e.Duration), e.Timestamp)}
	case *StageFailedEvent:
		return []events.Event{logEvent("error", fmt.Sprintf("stage %s failed for bead %s (iteration %d): %s", e.StageName, e.BeadID, e.Iteration, e.Error), e.Timestamp)}
	case *StageRetryingEvent:
		return []events.Event{&events.StageRetryingEvent{
			StageName: e.StageName,
			Attempt:   e.Attempt,
			Reason:    e.Reason,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *ValidationEvent:
		return convertValidationEvent(e)
	case *ReviewEvent:
		return []events.Event{&events.ReviewCompleteEvent{
			BeadID:  e.BeadID,
			Verdict: e.Verdict,
			Issues:  append([]string(nil), e.Issues...),
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *ScopeEvent:
		return []events.Event{&events.ScopeCheckEvent{
			BeadID:     e.BeadID,
			Complexity: e.Complexity,
			Approved:   e.Approved,
			Reason:     e.Reason,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	case *TelemetryEvent:
		return []events.Event{logEvent("info", fmt.Sprintf("telemetry for bead %s stage %s: duration=%v input=%d output=%d cost=%.2f category=%s", e.BeadID, e.StageName, e.Duration, e.InputTokens, e.OutputTokens, e.CostUSD, e.Category), e.Timestamp)}
	default:
		return nil
	}
}

func convertBeadCompleted(e *BeadCompletedEvent) []events.Event {
	base := []events.Event{&events.IterationCompleteEvent{
		Iteration: e.Iteration,
		BeadID:    e.BeadID,
		Success:   e.Success,
		Duration:  0,
		TimeMixin: events.TimeMixin{
			Time: e.Timestamp,
		},
	}}
	if e.Success {
		return append(base, &events.BeadCompleteEvent{
			BeadID:    e.BeadID,
			BeadTitle: e.BeadTitle,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		})
	}
	return append(base, &events.BeadFailedEvent{
		BeadID:    e.BeadID,
		BeadTitle: e.BeadTitle,
		Error:     fmt.Sprintf("iteration %d failed", e.Iteration),
		TimeMixin: events.TimeMixin{
			Time: e.Timestamp,
		},
	})
}

func convertValidationEvent(e *ValidationEvent) []events.Event {
	if e.Succeeded {
		return []events.Event{&events.ValidationPassEvent{
			BeadID:   e.BeadID,
			Duration: e.Duration,
			TimeMixin: events.TimeMixin{
				Time: e.Timestamp,
			},
		}}
	}
	output := e.Details
	if output == "" {
		output = e.FailedCommand
	}
	return []events.Event{&events.ValidationFailEvent{
		BeadID:   e.BeadID,
		Output:   output,
		Duration: e.Duration,
		TimeMixin: events.TimeMixin{
			Time: e.Timestamp,
		},
	}}
}

func logEvent(level, message string, timestamp time.Time) events.Event {
	return &events.LogEvent{
		Level:   level,
		Message: message,
		TimeMixin: events.TimeMixin{
			Time: timestamp,
		},
	}
}
