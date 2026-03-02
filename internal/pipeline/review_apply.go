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
		labels := append([]string(nil), review.BuildReviewBeadLabels(bp.Labels)...)
		outputs := append([]string(nil), review.ExpectedOutputsOrTitle(bp.ExpectedOutputs, bp.Title)...)
        bead, err := p.deps.TrackerClient.Create(ctx, bp.Title, bp.Priority, labels, outputs)
        if err != nil {
            return nil, fmt.Errorf("creating review bead %q: %w", bp.Title, err)
        }
        if bead == nil {
            return nil, fmt.Errorf("creating review bead %q: tracker returned no bead", bp.Title)
        }
        applyResult.CreatedBeadIDs = append(applyResult.CreatedBeadIDs, bead.ID)
    }

	backlogCount, err := applyBacklogItems(ctx, result.BacklogItems, p.deps.BacklogWriter)
	if err != nil {
		return nil, err
	}
	applyResult.CreatedBacklogCount = backlogCount

	learningsSaved, err := persistLearnings(result.Learnings, p.deps.LearningsManager)
	if err != nil {
		return nil, err
	}
	applyResult.LearningsSaved = learningsSaved

	return &applyResult, nil
}

func buildReviewBacklogEntry(item review.BacklogItem) *BacklogEntry {
	description := item.Description
	if item.Reason != "" {
		if description != "" {
			description += "\n\n"
		}
		description += "Reason for backlog: " + item.Reason
	}

	return &BacklogEntry{
		Title:           item.Title,
		Type:            backlogTypeReviewFinding,
		Description:     description,
		Priority:        backlogPriorityDefault,
		Labels:          review.BuildBacklogLabels(),
		ExpectedOutputs: review.ExpectedOutputsOrTitle(item.ExpectedOutputs, item.Title),
	}
}

func applyBacklogItems(ctx context.Context, items []review.BacklogItem, writer BacklogWriter) (int, error) {
	count := 0
	for _, bi := range items {
		entry := buildReviewBacklogEntry(bi)
		if err := writer.Add(ctx, entry); err != nil {
			return count, fmt.Errorf("creating backlog item %q: %w", bi.Title, err)
		}
		count++
	}
	return count, nil
}

func persistLearnings(learnings []string, manager LearningsManager) (int, error) {
	count := 0
	for _, learning := range learnings {
		if err := manager.Add(learning); err != nil {
			return count, fmt.Errorf("persisting learning: %w", err)
		}
		count++
	}
	return count, nil
}
