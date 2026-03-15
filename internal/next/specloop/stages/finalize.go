package stages

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// FinalizeStage determines the terminal status and handles worktree cleanup.
type FinalizeStage struct {
	gitOps   GitOps
	store    *runstore.Store
	eventLog *runstore.EventLog
}

// NewFinalizeStage creates a new FinalizeStage.
func NewFinalizeStage(gitOps GitOps, store *runstore.Store, eventLog *runstore.EventLog) *FinalizeStage {
	return &FinalizeStage{gitOps: gitOps, store: store, eventLog: eventLog}
}

// Name returns the stage name.
func (s *FinalizeStage) Name() string { return "finalize" }

// Run determines the terminal status and saves the final run state.
func (s *FinalizeStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// If already in a terminal state (e.g., blocked), preserve worktree and finalize
	if rs.Status == runstore.StatusBlocked {
		// Emit terminal_state event for blocked
		if s.eventLog != nil {
			s.eventLog.Append(runstore.TerminalStateEvent{
				BaseEvent: runstore.BaseEvent{Type: "terminal_state", Timestamp: time.Now()},
				Status:    rs.Status,
				Reason:    rs.TerminalReason,
			})
		}
		rs.EndedAt = time.Now()
		if err := s.store.Save(rs); err != nil {
			return specloop.NextAction{}, fmt.Errorf("save run state: %w", err)
		}
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	// Determine terminal status by the three quality gates.
	// Individual task failures from earlier cycles do not block ready_for_review
	// if validation, review, and acceptance all passed in the final cycle.
	if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
		rs.Status = runstore.StatusReadyForReview
	} else {
		rs.Status = runstore.StatusNeedsHuman
	}

	// Emit terminal_state event
	if s.eventLog != nil {
		s.eventLog.Append(runstore.TerminalStateEvent{
			BaseEvent: runstore.BaseEvent{Type: "terminal_state", Timestamp: time.Now()},
			Status:    rs.Status,
			Reason:    rs.TerminalReason,
		})
	}

	// Preserve worktree for ready_for_review and needs_human
	rs.EndedAt = time.Now()
	if err := s.store.Save(rs); err != nil {
		return specloop.NextAction{}, fmt.Errorf("save run state: %w", err)
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
