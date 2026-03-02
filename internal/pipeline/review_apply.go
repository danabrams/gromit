package pipeline

import (
    "context"
    "fmt"

    "github.com/danabrams/gromit/internal/review"
)

// ApplyReviewFindings creates beads, backlog entries, and learnings from the given review result.
// It returns a ReviewApplyResult that describes the artifacts created.
func (p *Pipeline) ApplyReviewFindings(ctx context.Context, result *review.ReviewResult) (*ReviewApplyResult, error) {
    if p == nil || p.deps == nil {
        return nil, fmt.Errorf("pipeline: nil dependencies")
    }
    if result == nil {
        empty := NewReviewApplyResult()
        return &empty, nil
    }

    if err := requireNonNilDep("TrackerClient", p.deps.TrackerClient); err != nil {
        return nil, err
    }
    if err := requireNonNilDep("BacklogWriter", p.deps.BacklogWriter); err != nil {
        return nil, err
    }
    if err := requireNonNilDep("LearningsManager", p.deps.LearningsManager); err != nil {
        return nil, err
    }

    applyResult := NewReviewApplyResult()

    for _, bp := range result.BeadsToCreate {
        labels := review.BuildReviewBeadLabels(bp.Labels)
        outputs := review.ExpectedOutputsOrTitle(bp.ExpectedOutputs, bp.Title)
        bead, err := p.deps.TrackerClient.Create(ctx, bp.Title, bp.Priority, labels, outputs)
        if err != nil {
            return nil, fmt.Errorf("creating review bead %q: %w", bp.Title, err)
        }
        applyResult.CreatedBeadIDs = append(applyResult.CreatedBeadIDs, bead.ID)
    }

    for _, bi := range result.BacklogItems {
        description := bi.Description
        if bi.Reason != "" {
            if description != "" {
                description += "\n\n"
            }
            description += "Reason for backlog: " + bi.Reason
        }
        entry := &BacklogEntry{
            Title:           bi.Title,
            Type:            backlogTypeReviewFinding,
            Description:     description,
            Priority:        backlogPriorityDefault,
            Labels:          review.BuildBacklogLabels(),
            ExpectedOutputs: review.ExpectedOutputsOrTitle(bi.ExpectedOutputs, bi.Title),
        }
        if err := p.deps.BacklogWriter.Add(ctx, entry); err != nil {
            return nil, fmt.Errorf("creating backlog item %q: %w", bi.Title, err)
        }
        applyResult.CreatedBacklogCount++
    }

    for _, learning := range result.Learnings {
        if err := p.deps.LearningsManager.Add(learning); err != nil {
            return nil, fmt.Errorf("persisting learning: %w", err)
        }
        applyResult.LearningsSaved++
    }

    return &applyResult, nil
}
