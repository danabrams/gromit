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
		return legacySpecStarted(e)
	case SpecStartedEvent:
		return legacySpecStarted(&e)
	case *SpecCompletedEvent:
		return legacySpecCompleted(e)
	case SpecCompletedEvent:
		return legacySpecCompleted(&e)
	case *SpecFailedEvent:
		return legacySpecFailed(e)
	case SpecFailedEvent:
		return legacySpecFailed(&e)
	case *BeadStartedEvent:
		return legacyBeadStarted(e)
	case BeadStartedEvent:
		return legacyBeadStarted(&e)
	case *BeadCompletedEvent:
		return convertBeadCompleted(e)
	case BeadCompletedEvent:
		return convertBeadCompleted(&e)
	case *StageStartedEvent:
		return legacyStageStarted(e)
	case StageStartedEvent:
		return legacyStageStarted(&e)
	case *StageCompletedEvent:
		return legacyStageCompleted(e)
	case StageCompletedEvent:
		return legacyStageCompleted(&e)
	case *StageFailedEvent:
		return legacyStageFailed(e)
	case StageFailedEvent:
		return legacyStageFailed(&e)
	case *StageRetryingEvent:
		return legacyStageRetrying(e)
	case StageRetryingEvent:
		return legacyStageRetrying(&e)
	case *ValidationEvent:
		return convertValidationEvent(e)
	case ValidationEvent:
		return convertValidationEvent(&e)
	case *ReviewEvent:
		return legacyReview(e)
	case ReviewEvent:
		return legacyReview(&e)
	case *ScopeEvent:
		return legacyScope(e)
	case ScopeEvent:
		return legacyScope(&e)
	case *TelemetryEvent:
		return legacyTelemetry(e)
	case TelemetryEvent:
		return legacyTelemetry(&e)
	case *GenerationCapReachedEvent:
		return legacyGenerationCapReached(e)
	case GenerationCapReachedEvent:
		return legacyGenerationCapReached(&e)
	default:
		return nil
	}
}

// BridgeTypedToLegacy wires the typed emitter to the legacy emitter, converting events before re-emitting.
func BridgeTypedToLegacy(typed *Emitter, legacy *events.Emitter) {
	if typed == nil || legacy == nil {
		return
	}
	typed.Subscribe(func(evt TypedEvent) {
		for _, legacyEvt := range legacyEventsFromTyped(evt) {
			if legacyEvt == nil {
				continue
			}
			legacy.Emit(legacyEvt)
		}
	})
}

func legacySpecStarted(e *SpecStartedEvent) []events.Event {
	return []events.Event{&events.SpecStartedEvent{
		SpecID:    e.SpecID,
		Worktree:  e.Worktree,
		TimeMixin: toTimeMixin(e.Timestamp),
	}}
}

func legacySpecCompleted(e *SpecCompletedEvent) []events.Event {
	return []events.Event{&events.SpecCompletedEvent{
		SpecID:        e.SpecID,
		Worktree:      e.Worktree,
		Success:       e.Success,
		FailureReason: e.FailureReason,
		TimeMixin:     toTimeMixin(e.Timestamp),
	}}
}

func legacySpecFailed(e *SpecFailedEvent) []events.Event {
	return []events.Event{&events.SpecFailedEvent{
		SpecID:        e.SpecID,
		Worktree:      e.Worktree,
		FailureReason: e.FailureReason,
		TimeMixin:     toTimeMixin(e.Timestamp),
	}}
}

func legacyBeadStarted(e *BeadStartedEvent) []events.Event {
	return []events.Event{&events.IterationStartEvent{
		Iteration: e.Iteration,
		BeadID:    e.BeadID,
		BeadTitle: e.BeadTitle,
		TimeMixin: toTimeMixin(e.Timestamp),
	}}
}

func legacyStageStarted(e *StageStartedEvent) []events.Event {
	return []events.Event{logEvent("info", fmt.Sprintf("stage %s started for bead %s (iteration %d)", e.StageName, e.BeadID, e.Iteration), e.Timestamp)}
}

func legacyStageCompleted(e *StageCompletedEvent) []events.Event {
	status := "completed"
	if !e.Success {
		status = "failed"
	}
	return []events.Event{logEvent("info", fmt.Sprintf("stage %s %s for bead %s (iteration %d, duration %v)", e.StageName, status, e.BeadID, e.Iteration, e.Duration), e.Timestamp)}
}

func legacyStageFailed(e *StageFailedEvent) []events.Event {
	return []events.Event{logEvent("error", fmt.Sprintf("stage %s failed for bead %s (iteration %d): %s", e.StageName, e.BeadID, e.Iteration, e.Error), e.Timestamp)}
}

func legacyStageRetrying(e *StageRetryingEvent) []events.Event {
	return []events.Event{&events.StageRetryingEvent{
		StageName: e.StageName,
		Attempt:   e.Attempt,
		Reason:    e.Reason,
		TimeMixin: toTimeMixin(e.Timestamp),
	}}
}

func legacyReview(e *ReviewEvent) []events.Event {
	return []events.Event{&events.ReviewCompleteEvent{
		BeadID:    e.BeadID,
		Verdict:   e.Verdict,
		Issues:    append([]string(nil), e.Issues...),
		TimeMixin: toTimeMixin(e.Timestamp),
	}}
}

func legacyScope(e *ScopeEvent) []events.Event {
	return []events.Event{&events.ScopeCheckEvent{
		BeadID:     e.BeadID,
		Complexity: e.Complexity,
		Approved:   e.Approved,
		Reason:     e.Reason,
		TimeMixin:  toTimeMixin(e.Timestamp),
	}}
}

func legacyTelemetry(e *TelemetryEvent) []events.Event {
	return []events.Event{logEvent("info", fmt.Sprintf("telemetry for bead %s stage %s: duration=%v input=%d output=%d cost=%.2f category=%s", e.BeadID, e.StageName, e.Duration, e.InputTokens, e.OutputTokens, e.CostUSD, e.Category), e.Timestamp)}
}

func toTimeMixin(ts time.Time) events.TimeMixin {
	return events.TimeMixin{Time: ts}
}

func convertBeadCompleted(e *BeadCompletedEvent) []events.Event {
	base := []events.Event{&events.IterationCompleteEvent{
		Iteration: e.Iteration,
		BeadID:    e.BeadID,
		Success:   e.Success,
		Duration:  0,
		TimeMixin: toTimeMixin(e.Timestamp),
	}}
	if e.Success {
		return append(base, &events.BeadCompleteEvent{
			BeadID:    e.BeadID,
			BeadTitle: e.BeadTitle,
			TimeMixin: toTimeMixin(e.Timestamp),
		})
	}
	return append(base, &events.BeadFailedEvent{
		BeadID:    e.BeadID,
		BeadTitle: e.BeadTitle,
		Error:     fmt.Sprintf("iteration %d failed", e.Iteration),
		TimeMixin: toTimeMixin(e.Timestamp),
	})
}

func convertValidationEvent(e *ValidationEvent) []events.Event {
	if e.Succeeded {
		return []events.Event{&events.ValidationPassEvent{
			BeadID:    e.BeadID,
			Duration:  e.Duration,
			TimeMixin: toTimeMixin(e.Timestamp),
		}}
	}
	output := e.Details
	if output == "" {
		output = e.FailedCommand
	}
	return []events.Event{&events.ValidationFailEvent{
		BeadID:    e.BeadID,
		Output:    output,
		Duration:  e.Duration,
		TimeMixin: toTimeMixin(e.Timestamp),
	}}
}

func legacyGenerationCapReached(e *GenerationCapReachedEvent) []events.Event {
	return []events.Event{&events.GenerationCapReachedEvent{
		GenerationCap: e.GenerationCap,
		TimeMixin:     toTimeMixin(e.Timestamp),
	}}
}

func logEvent(level, message string, timestamp time.Time) events.Event {
	return &events.LogEvent{
		Level:     level,
		Message:   message,
		TimeMixin: toTimeMixin(timestamp),
	}
}
