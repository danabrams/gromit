package reviewsession

import (
	"time"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
)

// Result constants for checklist items.
const (
	ResultPending = "pending"
	ResultPass    = "pass"
	ResultFail    = "fail"
	ResultUnsure  = "unsure"
	ResultSkipped = "skipped"
)

// Outcome constants for review decisions.
const (
	OutcomeAccepted                = "accepted"
	OutcomeReworkImplementationGap = "rework_implementation_gap"
	OutcomeReworkVisionChange      = "rework_vision_change"
)

// Session manages the review as a step-by-step protocol.
type Session struct {
	Packet      reviewpacket.Outputs `json:"packet"`
	Checklist   []ChecklistItemState `json:"checklist"`
	CurrentStep int                  `json:"current_step"`
	Outcome     *ReviewOutcome       `json:"outcome,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values.
func (s *Session) NormalizeNilFields() {
	if s.Checklist == nil {
		s.Checklist = []ChecklistItemState{}
	}
	for i := range s.Checklist {
		s.Checklist[i].NormalizeNilFields()
	}
	if s.Outcome != nil {
		s.Outcome.NormalizeNilFields()
	}
}

// ChecklistItemState tracks the review result for a manual checklist item.
type ChecklistItemState struct {
	Item   reviewpacket.ManualCheckItem `json:"item"`
	Result string                       `json:"result"` // pending, pass, fail, unsure, skipped
	Notes  string                       `json:"notes"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values.
func (c *ChecklistItemState) NormalizeNilFields() {
	// ChecklistItemState has no nil slices/maps
}

// ReviewOutcome records the final review decision and its details.
type ReviewOutcome struct {
	RunID          string              `json:"run_id"`
	ReviewedAt     time.Time           `json:"reviewed_at"`
	Outcome        string              `json:"outcome"`
	Summary        string              `json:"summary"`
	ManualResults  []ManualCheckResult `json:"manual_results,omitempty"`
	OverrideReason string              `json:"override_reason,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values.
func (ro *ReviewOutcome) NormalizeNilFields() {
	if ro.ManualResults == nil {
		ro.ManualResults = []ManualCheckResult{}
	}
}

// ManualCheckResult records the reviewer's result for a single manual check.
type ManualCheckResult struct {
	ID     string `json:"id"`
	Result string `json:"result"`
	Notes  string `json:"notes,omitempty"`
}
