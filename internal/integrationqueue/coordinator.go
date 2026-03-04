package integrationqueue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

const mergedTransitionReason = "coordinator: merged"

// ScopedGate performs the scoped gate checks required before integration.
type ScopedGate interface {
	Run(ctx context.Context, entry Entry) error
}

// Coordinator drives the integration queue lifecycle for a single branch per cycle.
type Coordinator struct {
	store        *Store
	gitops       GitOps
	gate         ScopedGate
	transitionFn func(entry *Entry, toState string, reason string, metadata ...TransitionErrorMetadata) error
}

// NewCoordinator wires together the store, gitops adapter, and scoped gate runner.
func NewCoordinator(store *Store, gitops GitOps, gate ScopedGate) *Coordinator {
	return &Coordinator{store: store, gitops: gitops, gate: gate}
}

func (c *Coordinator) applyTransition(entry *Entry, toState string, reason string, metadata ...TransitionErrorMetadata) error {
	fn := c.transitionFn
	if fn == nil {
		fn = ApplyTransition
	}
	return fn(entry, toState, reason, metadata...)
}

// Coordinate processes at most one ready branch in FIFO order.
func (c *Coordinator) Coordinate(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("store is required")
	}
	if c.gitops == nil {
		return fmt.Errorf("gitops adapter is required")
	}
	if c.gate == nil {
		return fmt.Errorf("scoped gate runner is required")
	}

	queue, err := c.store.load()
	if err != nil {
		return fmt.Errorf("loading integration queue: %w", err)
	}

	entry := OldestReady(queue)
	if entry == nil {
		return nil
	}

	entry.AttemptCount++
	entry.RetryCount = 0
	if err := c.applyTransition(entry, string(StateIntegrating), "coordinator: starting integration"); err != nil {
		return fmt.Errorf("transitioning entry to integrating: %w", err)
	}
	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("marking entry integrating: %w", err)
	}

	if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
		code := classifyFetchAndRebaseErrorCode(err)
		message := err.Error()
		combinedErr := err

		if transErr := c.applyTransition(entry, string(StateConflict), fetchAndRebaseFailureReason(code, false), TransitionErrorMetadata{Code: code, Message: message}); transErr != nil {
			combinedErr = errors.Join(combinedErr, transErr)
			return fmt.Errorf("fetch/rebase branch: %w", combinedErr)
		}

		if saveErr := c.store.Save(*entry); saveErr != nil {
			combinedErr = errors.Join(combinedErr, saveErr)
		}

		return fmt.Errorf("fetch/rebase branch: %w", combinedErr)
	}

	if err := c.gate.Run(ctx, *entry); err != nil {
		entry.RetryCount = 1
		if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
			code := classifyFetchAndRebaseErrorCode(err)
			message := err.Error()
			combinedErr := err

			if transErr := c.applyTransition(entry, string(StateConflict), fetchAndRebaseFailureReason(code, true), TransitionErrorMetadata{Code: code, Message: message}); transErr != nil {
				combinedErr = errors.Join(combinedErr, transErr)
				return fmt.Errorf("fetch/rebase branch: %w", combinedErr)
			}

			if saveErr := c.store.Save(*entry); saveErr != nil {
				combinedErr = errors.Join(combinedErr, saveErr)
			}

			return fmt.Errorf("fetch/rebase branch: %w", combinedErr)
		}
		if err := c.gate.Run(ctx, *entry); err != nil {
			entry.RetryCount++
			metadata := TransitionErrorMetadata{Code: string(StateFailedGates), Message: err.Error()}
			if transErr := c.applyTransition(entry, string(StateFailedGates), "scoped gates failed after retry", metadata); transErr == nil {
				if saveErr := c.store.Save(*entry); saveErr != nil {
					return fmt.Errorf("marking entry failed_gates: %w", saveErr)
				}
			}
			return fmt.Errorf("running scoped gates: %w", err)
		}
	}

	if err := c.gitops.MergeToMain(ctx, *entry); err != nil {
		targetState := StateConflict
		reason := "merge conflict detected"
		metadata := TransitionErrorMetadata{Message: err.Error()}
		if strings.Contains(strings.ToLower(err.Error()), "lane violation") {
			targetState = StateLaneViolation
			reason = "lane violation detected"
			metadata.Code = "lane_violation"
		} else {
			metadata.Code = "merge_conflict"
		}

		if transErr := c.applyTransition(entry, string(targetState), reason, metadata); transErr != nil {
			return fmt.Errorf("transitioning entry to %s: %w", targetState, transErr)
		}

		if saveErr := c.store.Save(*entry); saveErr != nil {
			return fmt.Errorf("marking entry conflict: %w", saveErr)
		}
		return fmt.Errorf("merging branch: %w", err)
	}

	if err := c.gitops.Push(ctx); err != nil {
		metadata := TransitionErrorMetadata{Code: "push_failed", Message: err.Error()}
		if transErr := c.applyTransition(entry, string(StatePushFailure), "push to remote failed", metadata); transErr != nil {
			return fmt.Errorf("transitioning entry to push failure: %w", transErr)
		}
		if saveErr := c.store.Save(*entry); saveErr != nil {
			return fmt.Errorf("marking entry push failure: %w", saveErr)
		}
		return fmt.Errorf("pushing main: %w", err)
	}

	cleanupErr := c.gitops.Cleanup(ctx, *entry)

	entry.ChangedFiles = nil
	entry.ChangedFilesHash = ""
	if transErr := c.applyTransition(entry, string(StateMerged), mergedTransitionReason, TransitionErrorMetadata{}); transErr != nil {
		return fmt.Errorf("applying merged transition: %w", transErr)
	}

	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("finalizing entry: %w", err)
	}

	if cleanupErr != nil {
		return fmt.Errorf("cleaning up metadata: %w", cleanupErr)
	}

	return nil
}

// RecoverFromCrash detects entries left in StateIntegrating and transitions
// them back to StateReady. This is called during startup to handle entries
// that were stranded mid-integration by a prior crash.
func (c *Coordinator) RecoverFromCrash(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("store is required")
	}

	queue, err := c.store.load()
	if err != nil {
		return fmt.Errorf("loading integration queue for recovery: %w", err)
	}

	// Find and reset any entries in integrating state
	recovered := false
	for i := range queue.Entries {
		entry := &queue.Entries[i]
		if entry.State == StateIntegrating {
			targetState := StateReady
			reason := "crash recovery"
			clearError := true

			switch entry.LastErrorCode {
			case string(StateFailedGates):
				targetState = StateFailedGates
				reason = "crash recovery: failed gates"
				clearError = false
			case "merge_conflict":
				targetState = StateConflict
				reason = "crash recovery: merge conflict"
				clearError = false
			case "push_failed":
				targetState = StatePushFailure
				reason = "crash recovery: push failure"
				clearError = false
			}

			metadata := []TransitionErrorMetadata{}
			if clearError {
				metadata = append(metadata, crashRecoveryMetadata())
			} else {
				metadata = append(metadata, TransitionErrorMetadata{Code: entry.LastErrorCode, Message: entry.LastErrorMessage})
			}

			transitionErr := c.applyTransition(entry, string(targetState), reason, metadata...)
			if transitionErr != nil {
				return fmt.Errorf("transitioning recovered entry %s: %w", entry.Branch, transitionErr)
			}

			// Save each recovered entry
			if err := c.store.Save(*entry); err != nil {
				return fmt.Errorf("saving recovered entry %s: %w", entry.Branch, err)
			}
			recovered = true
		}
	}

	// If no entries needed recovery, just log and return early
	if !recovered {
		log.Println("no stranded entries found during crash recovery")
	}

	return nil
}

// CrashRecoveryErrorCode identifies crash recovery metadata persisted when an
// integrating entry is reset to ready during startup recovery.
const (
	// CrashRecoveryErrorCode identifies crash recovery metadata persisted when an
	// integrating entry is reset to ready during startup recovery.
	CrashRecoveryErrorCode ErrorCode = "crash_recovery"
	crashRecoveryMessage             = "recovered from crash: entry was in integrating state"
)

func crashRecoveryMetadata() TransitionErrorMetadata {
	return TransitionErrorMetadata{
		Code:    string(CrashRecoveryErrorCode),
		Message: crashRecoveryMessage,
	}
}

func classifyFetchAndRebaseErrorCode(err error) string {
	if err == nil {
		return "rebase_conflict"
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "checkout branch") || strings.Contains(msg, " checkout ") {
		return "checkout_failed"
	}
	return "rebase_conflict"
}

func fetchAndRebaseFailureReason(errorCode string, retry bool) string {
	if errorCode == "checkout_failed" {
		if retry {
			return "checkout failed during retry fetch"
		}
		return "checkout failed during initial fetch"
	}
	if retry {
		return "rebase conflict during retry fetch"
	}
	return "rebase conflict during initial fetch"
}
