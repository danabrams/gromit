package loop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/events"
	gitpkg "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

type worktreeBranchRemover interface {
	RemoveWorktreeAndBranch(context.Context, string) error
}

func (s *SpecLoop) handleFailure(ctx context.Context, specID string, base presentation.PresentationSummary, failure error) error {
	s.recordStage("gap-analysis")
	s.recordStage("decompose")
	s.recordStage("bead-loop")

	if errors.Is(failure, ErrGenerationCapReached) {
		s.emitGenerationCapReached()
	}

	reason := fmt.Sprintf("spec %s remediation halted: %s", specID, failure.Error())
	s.emit(&events.AndonTriggeredEvent{SpecID: specID, Reason: reason})
	s.emit(&events.SpecFailedEvent{SpecID: specID, Worktree: base.Worktree, FailureReason: reason})

	summary := base
	summary.Success = false
	summary.RemainingWork = nil
	summary.FailureSummary = reason
	resultErr := fmt.Errorf("accept failure: %w", failure)

	gapSummary, err := s.readGapAnalysis(base.Worktree)
	if err != nil {
		resultErr = errors.Join(resultErr, fmt.Errorf("read gap analysis: %w", err))
	} else if gapSummary != "" {
		summary.FailureSummary = fmt.Sprintf("%s\n\nGap analysis:\n%s", reason, gapSummary)
		summary.RemainingWork = []string{gapSummary}
	}

	if err := s.presentSummary(ctx, specID, summary); err != nil {
		resultErr = errors.Join(resultErr, fmt.Errorf("present failure summary: %w", err))
	}

	cleanupOpts := cleanupOptions{reason: cleanupReasonAndon, forcePreserveBranch: true}
	if errors.Is(failure, ErrGenerationCapReached) {
		cleanupOpts.reason = cleanupReasonGenerationCap
	}
	if err := s.cleanupWorktree(ctx, specID, base.Worktree, false, cleanupOpts); err != nil {
		resultErr = errors.Join(resultErr, err)
	}

	s.emit(&events.SpecCompletedEvent{
		SpecID:        specID,
		Worktree:      base.Worktree,
		Success:       false,
		FailureReason: reason,
	})

	return resultErr
}

func (s *SpecLoop) emitGenerationCapReached() {
	if s.typedEmitter == nil {
		return
	}
	s.typedEmitter.Emit(event.GenerationCapReachedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeGenerationCapReached,
		},
	})
}

type cleanupReason int

const (
	cleanupReasonUnknown cleanupReason = iota
	cleanupReasonAndon
	cleanupReasonGenerationCap
	cleanupReasonSuccess
)

func (r cleanupReason) description() string {
	switch r {
	case cleanupReasonAndon:
		return "Andon triggered"
	case cleanupReasonGenerationCap:
		return "generation cap reached"
	case cleanupReasonSuccess:
		return "successful PR merge"
	default:
		return ""
	}
}

type cleanupOptions struct {
	reason              cleanupReason
	forcePreserveBranch bool
}

func (opts cleanupOptions) reasonString() string {
	return opts.reason.description()
}

func (s *SpecLoop) cleanupWorktree(_ context.Context, specID, worktree string, success bool, opts cleanupOptions) error {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		log.Printf("worktree cleanup skipped for spec %s: empty worktree path", specID)
		return nil
	}
	git := s.adapters.Git
	if git == nil {
		return fmt.Errorf("git adapter required for cleanup")
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reason := opts.reasonString()
	branchMgr, hasBranchMgr := git.(branchManager)
	if !success {
		status, err := git.Status(cleanupCtx, trimmed)
		if err != nil {
			log.Printf("git status during cleanup of spec %s: %v", specID, err)
		} else if strings.TrimSpace(status) != "" {
			message := fmt.Sprintf("[gromit: partial work] spec %s", specID)
			hash, err := git.Commit(cleanupCtx, trimmed, message)
			if err != nil {
				log.Printf("commit partial work for spec %s: %v", specID, err)
			} else {
				log.Printf("committed partial work for spec %s at %s", specID, strings.TrimSpace(hash))
			}
		}
		if opts.forcePreserveBranch || s.preserveOnFailure {
			log.Print(gitpkg.PreserveBranchMessage(specID, trimmed, reason))
			if hasBranchMgr {
				if err := branchMgr.PreserveBranch(cleanupCtx, specID); err != nil {
					return fmt.Errorf("preserve branch: %w", err)
				}
			} else {
				log.Printf("branch preservation requested for spec %s but branch manager is not configured", specID)
			}
			return nil
		}
		log.Print(gitpkg.RemoveFailedWorktreeMessage(specID, trimmed, reason))
	} else {
		if hasBranchMgr {
			log.Print(gitpkg.DeleteBranchMessage(specID, trimmed, reason))
		} else {
			log.Print(gitpkg.RemoveWorktreeMessage(specID, trimmed, reason))
		}
	}
	if err := git.RemoveWorktree(cleanupCtx, trimmed); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	if success && hasBranchMgr {
		if err := branchMgr.DeleteBranch(cleanupCtx, specID); err != nil {
			return fmt.Errorf("delete branch: %w", err)
		}
	}
	return nil
}
