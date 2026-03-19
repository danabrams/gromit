package reviewsession

import (
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
)

// Start initializes a session from a review packet.
func Start(packet reviewpacket.Outputs) *Session {
	s := &Session{
		Packet:    packet,
		Checklist: make([]ChecklistItemState, len(packet.ManualChecklist.Items)),
	}
	for i, item := range packet.ManualChecklist.Items {
		s.Checklist[i] = ChecklistItemState{
			Item:   item,
			Result: ResultPending,
		}
	}
	return s
}

// CurrentItem returns the current checklist item, or nil if all are done.
func (s *Session) CurrentItem() *ChecklistItemState {
	if s.CurrentStep >= len(s.Checklist) {
		return nil
	}
	return &s.Checklist[s.CurrentStep]
}

// RecordItemResult records the result for the current item and advances.
func (s *Session) RecordItemResult(result string, notes string) error {
	item := s.CurrentItem()
	if item == nil {
		return fmt.Errorf("no current item to record result for")
	}

	// Validate result value
	validResults := map[string]bool{
		ResultPending: true,
		ResultPass:    true,
		ResultFail:    true,
		ResultUnsure:  true,
		ResultSkipped: true,
	}
	if !validResults[result] {
		return fmt.Errorf("invalid result %q", result)
	}

	item.Result = result
	item.Notes = notes
	s.CurrentStep++
	return nil
}

// SkipRemaining marks all remaining items as skipped.
func (s *Session) SkipRemaining() {
	for i := s.CurrentStep; i < len(s.Checklist); i++ {
		s.Checklist[i].Result = ResultSkipped
	}
	s.CurrentStep = len(s.Checklist)
}

// CanAccept returns true if acceptance is valid given current checklist state.
// Returns a reason string if acceptance is not valid.
func (s *Session) CanAccept() (bool, string) {
	for _, item := range s.Checklist {
		if item.Result == ResultFail {
			return false, "acceptance not allowed with failed items"
		}
	}
	return true, ""
}

// NeedsOverride returns true if accepting requires an override note
// (e.g., some items are unsure).
func (s *Session) NeedsOverride() bool {
	for _, item := range s.Checklist {
		if item.Result == ResultUnsure {
			return true
		}
	}
	return false
}

// RecordOutcome validates and records the final outcome.
func (s *Session) RecordOutcome(outcome string, summary string, overrideReason string) (*ReviewOutcome, error) {
	// Validate outcome value
	validOutcomes := map[string]bool{
		OutcomeAccepted:                true,
		OutcomeReworkImplementationGap: true,
		OutcomeReworkVisionChange:      true,
	}
	if !validOutcomes[outcome] {
		return nil, fmt.Errorf("invalid outcome %q", outcome)
	}

	// Apply outcome-specific validation rules
	switch outcome {
	case OutcomeAccepted:
		canAccept, reason := s.CanAccept()
		if !canAccept {
			return nil, fmt.Errorf("cannot accept: %s", reason)
		}
		if s.NeedsOverride() && overrideReason == "" {
			return nil, fmt.Errorf("acceptance with unsure items requires override reason")
		}

	case OutcomeReworkImplementationGap:
		// At least one item must be fail/unsure OR summary must be non-empty
		hasFlaggedItem := false
		for _, item := range s.Checklist {
			if item.Result == ResultFail || item.Result == ResultUnsure {
				hasFlaggedItem = true
				break
			}
		}
		if !hasFlaggedItem && summary == "" {
			return nil, fmt.Errorf("rework_implementation_gap requires at least one failed/unsure item or a non-empty summary")
		}

	case OutcomeReworkVisionChange:
		// Summary must be non-empty
		if summary == "" {
			return nil, fmt.Errorf("rework_vision_change requires a non-empty summary")
		}
	}

	// Build ManualResults slice
	manualResults := make([]ManualCheckResult, len(s.Checklist))
	for i, item := range s.Checklist {
		manualResults[i] = ManualCheckResult{
			ID:     item.Item.ID,
			Result: item.Result,
			Notes:  item.Notes,
		}
	}

	// Create and store the outcome
	reviewOutcome := &ReviewOutcome{
		RunID:         s.Packet.ProductReview.RunID,
		ReviewedAt:    time.Now(),
		Outcome:       outcome,
		Summary:       summary,
		ManualResults: manualResults,
	}

	if overrideReason != "" {
		reviewOutcome.OverrideReason = overrideReason
	}

	s.Outcome = reviewOutcome
	return reviewOutcome, nil
}
