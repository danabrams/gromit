package loop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

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

	if err := s.cleanupWorktree(ctx, specID, base.Worktree, false); err != nil {
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

func (s *SpecLoop) cleanupWorktree(_ context.Context, specID, worktree string, success bool) error {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		log.Printf("worktree cleanup skipped for spec %s: empty worktree path", specID)
		return nil
	}
	git := s.adapters.Git
	if git == nil {
		return fmt.Errorf("git adapter required for cleanup")
	}
	// Use a fresh context so cleanup succeeds even when the caller's context
	// has been cancelled (exec.CommandContext fails immediately otherwise).
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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
		if s.preserveOnFailure {
			log.Printf("preserving failed spec worktree branch for spec %s at %s", specID, trimmed)
			return nil
		}
		log.Printf("removing failed spec worktree for spec %s at %s (preserve_on_failure=false)", specID, trimmed)
	}
	if success {
		if remover, ok := git.(worktreeBranchRemover); ok {
			log.Printf("removing worktree and deleting branch after successful presentation for spec %s at %s", specID, trimmed)
			if err := remover.RemoveWorktreeAndBranch(cleanupCtx, trimmed); err != nil {
				return fmt.Errorf("remove worktree and branch: %w", err)
			}
			return nil
		}
		log.Printf("git adapter cannot delete branches; removing worktree only for successful spec %s at %s", specID, trimmed)
	}
	log.Printf("removing worktree for spec %s at %s", specID, trimmed)
	if err := git.RemoveWorktree(cleanupCtx, trimmed); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}
