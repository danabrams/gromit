package reviewsession

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
)

func TestStart(t *testing.T) {
	tests := []struct {
		name          string
		packet        reviewpacket.Outputs
		wantChecklist int
	}{
		{
			name: "empty checklist",
			packet: reviewpacket.Outputs{
				ManualChecklist: reviewpacket.ManualChecklist{
					Items: []reviewpacket.ManualCheckItem{},
				},
			},
			wantChecklist: 0,
		},
		{
			name: "single item",
			packet: reviewpacket.Outputs{
				ManualChecklist: reviewpacket.ManualChecklist{
					Items: []reviewpacket.ManualCheckItem{
						{
							ID:             "item1",
							Title:          "Check item",
							Instructions:   "Do this",
							ExpectedResult: "Success",
						},
					},
				},
			},
			wantChecklist: 1,
		},
		{
			name: "multiple items",
			packet: reviewpacket.Outputs{
				ManualChecklist: reviewpacket.ManualChecklist{
					Items: []reviewpacket.ManualCheckItem{
						{ID: "item1", Title: "Item 1"},
						{ID: "item2", Title: "Item 2"},
						{ID: "item3", Title: "Item 3"},
					},
				},
			},
			wantChecklist: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := Start(tt.packet)

			// Verify packet is stored
			if session.Packet.ManualChecklist.Items == nil {
				t.Fatal("Packet not stored correctly")
			}

			// Verify checklist length
			if len(session.Checklist) != tt.wantChecklist {
				t.Errorf("checklist length = %d, want %d", len(session.Checklist), tt.wantChecklist)
			}

			// Verify current step starts at 0
			if session.CurrentStep != 0 {
				t.Errorf("CurrentStep = %d, want 0", session.CurrentStep)
			}

			// Verify all items are initialized with pending status
			for i, itemState := range session.Checklist {
				if itemState.Result != ResultPending {
					t.Errorf("item %d result = %q, want %q", i, itemState.Result, ResultPending)
				}
				if itemState.Item.ID == "" {
					t.Errorf("item %d not initialized correctly", i)
				}
			}
		})
	}
}

func TestCurrentItem(t *testing.T) {
	packet := reviewpacket.Outputs{
		ManualChecklist: reviewpacket.ManualChecklist{
			Items: []reviewpacket.ManualCheckItem{
				{ID: "item1", Title: "Item 1"},
				{ID: "item2", Title: "Item 2"},
				{ID: "item3", Title: "Item 3"},
			},
		},
	}

	session := Start(packet)

	tests := []struct {
		name    string
		step    int
		wantID  string
		wantNil bool
	}{
		{
			name:   "first item",
			step:   0,
			wantID: "item1",
		},
		{
			name:   "second item",
			step:   1,
			wantID: "item2",
		},
		{
			name:   "third item",
			step:   2,
			wantID: "item3",
		},
		{
			name:    "past end",
			step:    3,
			wantNil: true,
		},
		{
			name:    "well past end",
			step:    100,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session.CurrentStep = tt.step
			item := session.CurrentItem()

			if tt.wantNil {
				if item != nil {
					t.Errorf("CurrentItem() = %v, want nil", item)
				}
			} else {
				if item == nil {
					t.Errorf("CurrentItem() = nil, want item")
				} else if item.Item.ID != tt.wantID {
					t.Errorf("item ID = %q, want %q", item.Item.ID, tt.wantID)
				}
			}
		})
	}
}

func TestRecordItemResult(t *testing.T) {
	packet := reviewpacket.Outputs{
		ManualChecklist: reviewpacket.ManualChecklist{
			Items: []reviewpacket.ManualCheckItem{
				{ID: "item1", Title: "Item 1"},
				{ID: "item2", Title: "Item 2"},
			},
		},
	}

	tests := []struct {
		name       string
		step       int
		result     string
		notes      string
		wantErr    bool
		wantResult string
	}{
		{
			name:       "valid pass result",
			step:       0,
			result:     ResultPass,
			notes:      "Passed verification",
			wantResult: ResultPass,
		},
		{
			name:       "valid fail result",
			step:       0,
			result:     ResultFail,
			notes:      "Failed verification",
			wantResult: ResultFail,
		},
		{
			name:       "valid unsure result",
			step:       0,
			result:     ResultUnsure,
			notes:      "Unable to determine",
			wantResult: ResultUnsure,
		},
		{
			name:       "valid skipped result",
			step:       0,
			result:     ResultSkipped,
			notes:      "Skipped",
			wantResult: ResultSkipped,
		},
		{
			name:    "invalid result",
			step:    0,
			result:  "invalid",
			wantErr: true,
		},
		{
			name:    "no current item",
			step:    2,
			result:  ResultPass,
			wantErr: true,
		},
		{
			name:       "empty notes",
			step:       0,
			result:     ResultPass,
			notes:      "",
			wantResult: ResultPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := Start(packet)
			session.CurrentStep = tt.step

			err := session.RecordItemResult(tt.result, tt.notes)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RecordItemResult() = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("RecordItemResult() = %v, want nil", err)
				}

				// Verify result was recorded
				if session.Checklist[tt.step].Result != tt.wantResult {
					t.Errorf("result = %q, want %q", session.Checklist[tt.step].Result, tt.wantResult)
				}

				// Verify notes were recorded
				if session.Checklist[tt.step].Notes != tt.notes {
					t.Errorf("notes = %q, want %q", session.Checklist[tt.step].Notes, tt.notes)
				}

				// Verify step advanced
				if session.CurrentStep != tt.step+1 {
					t.Errorf("CurrentStep = %d, want %d", session.CurrentStep, tt.step+1)
				}
			}
		})
	}
}

func TestSkipRemaining(t *testing.T) {
	packet := reviewpacket.Outputs{
		ManualChecklist: reviewpacket.ManualChecklist{
			Items: []reviewpacket.ManualCheckItem{
				{ID: "item1", Title: "Item 1"},
				{ID: "item2", Title: "Item 2"},
				{ID: "item3", Title: "Item 3"},
				{ID: "item4", Title: "Item 4"},
			},
		},
	}

	tests := []struct {
		name        string
		startStep   int
		wantStep    int
		wantSkipped map[int]bool // index -> should be skipped
	}{
		{
			name:      "skip from start",
			startStep: 0,
			wantStep:  4,
			wantSkipped: map[int]bool{
				0: true,
				1: true,
				2: true,
				3: true,
			},
		},
		{
			name:      "skip from middle",
			startStep: 2,
			wantStep:  4,
			wantSkipped: map[int]bool{
				2: true,
				3: true,
			},
		},
		{
			name:        "skip from end",
			startStep:   4,
			wantStep:    4,
			wantSkipped: map[int]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := Start(packet)
			session.CurrentStep = tt.startStep

			session.SkipRemaining()

			// Verify step advanced to end
			if session.CurrentStep != tt.wantStep {
				t.Errorf("CurrentStep = %d, want %d", session.CurrentStep, tt.wantStep)
			}

			// Verify skipped items
			for i := 0; i < len(session.Checklist); i++ {
				shouldSkip := tt.wantSkipped[i]
				isSkipped := session.Checklist[i].Result == ResultSkipped

				if shouldSkip && !isSkipped {
					t.Errorf("item %d should be skipped but is %q", i, session.Checklist[i].Result)
				}
				if !shouldSkip && isSkipped {
					t.Errorf("item %d should not be skipped", i)
				}
			}
		})
	}
}

func TestCanAccept(t *testing.T) {
	tests := []struct {
		name    string
		results []string
		can     bool
		reason  string
	}{
		{
			name:    "all pass",
			results: []string{ResultPass, ResultPass, ResultPass},
			can:     true,
		},
		{
			name:    "all pending",
			results: []string{ResultPending, ResultPending},
			can:     true,
		},
		{
			name:    "mix of pass and pending",
			results: []string{ResultPass, ResultPending, ResultPass},
			can:     true,
		},
		{
			name:    "with unsure",
			results: []string{ResultPass, ResultUnsure, ResultPass},
			can:     true,
		},
		{
			name:    "with skipped",
			results: []string{ResultPass, ResultSkipped, ResultPass},
			can:     true,
		},
		{
			name:    "single fail",
			results: []string{ResultFail},
			can:     false,
			reason:  "acceptance not allowed with failed items",
		},
		{
			name:    "fail in middle",
			results: []string{ResultPass, ResultFail, ResultPass},
			can:     false,
			reason:  "acceptance not allowed with failed items",
		},
		{
			name:    "multiple fails",
			results: []string{ResultFail, ResultFail},
			can:     false,
			reason:  "acceptance not allowed with failed items",
		},
		{
			name:    "empty",
			results: []string{},
			can:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := reviewpacket.Outputs{
				ManualChecklist: reviewpacket.ManualChecklist{
					Items: makeCheckItems(len(tt.results)),
				},
			}

			session := Start(packet)
			for i, result := range tt.results {
				session.Checklist[i].Result = result
			}

			can, reason := session.CanAccept()

			if can != tt.can {
				t.Errorf("can = %v, want %v", can, tt.can)
			}
			if reason != tt.reason {
				t.Errorf("reason = %q, want %q", reason, tt.reason)
			}
		})
	}
}

func TestNeedsOverride(t *testing.T) {
	tests := []struct {
		name    string
		results []string
		need    bool
	}{
		{
			name:    "no unsure",
			results: []string{ResultPass, ResultPass},
			need:    false,
		},
		{
			name:    "single unsure",
			results: []string{ResultUnsure},
			need:    true,
		},
		{
			name:    "unsure in middle",
			results: []string{ResultPass, ResultUnsure, ResultPass},
			need:    true,
		},
		{
			name:    "multiple unsure",
			results: []string{ResultUnsure, ResultUnsure},
			need:    true,
		},
		{
			name:    "with fail but no unsure",
			results: []string{ResultPass, ResultFail},
			need:    false,
		},
		{
			name:    "with skipped but no unsure",
			results: []string{ResultPass, ResultSkipped},
			need:    false,
		},
		{
			name:    "empty",
			results: []string{},
			need:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := reviewpacket.Outputs{
				ManualChecklist: reviewpacket.ManualChecklist{
					Items: makeCheckItems(len(tt.results)),
				},
			}

			session := Start(packet)
			for i, result := range tt.results {
				session.Checklist[i].Result = result
			}

			need := session.NeedsOverride()

			if need != tt.need {
				t.Errorf("NeedsOverride() = %v, want %v", need, tt.need)
			}
		})
	}
}

func TestRecordOutcome(t *testing.T) {
	tests := []struct {
		name            string
		results         []string
		outcome         string
		summary         string
		overrideReason  string
		wantErr         bool
		wantErrSubstr   string
		wantOutcomeType string
	}{
		{
			name:            "accepted with all pass",
			results:         []string{ResultPass, ResultPass},
			outcome:         OutcomeAccepted,
			summary:         "",
			overrideReason:  "",
			wantErr:         false,
			wantOutcomeType: OutcomeAccepted,
		},
		{
			name:            "accepted with unsure and override",
			results:         []string{ResultPass, ResultUnsure},
			outcome:         OutcomeAccepted,
			summary:         "",
			overrideReason:  "Reviewed manually",
			wantErr:         false,
			wantOutcomeType: OutcomeAccepted,
		},
		{
			name:           "accepted with unsure no override",
			results:        []string{ResultPass, ResultUnsure},
			outcome:        OutcomeAccepted,
			summary:        "",
			overrideReason: "",
			wantErr:        true,
			wantErrSubstr:  "unsure items requires override reason",
		},
		{
			name:          "accepted with fail",
			results:       []string{ResultPass, ResultFail},
			outcome:       OutcomeAccepted,
			wantErr:       true,
			wantErrSubstr: "failed items",
		},
		{
			name:            "rework_implementation_gap with fail",
			results:         []string{ResultPass, ResultFail},
			outcome:         OutcomeReworkImplementationGap,
			summary:         "",
			overrideReason:  "",
			wantErr:         false,
			wantOutcomeType: OutcomeReworkImplementationGap,
		},
		{
			name:            "rework_implementation_gap with unsure",
			results:         []string{ResultPass, ResultUnsure},
			outcome:         OutcomeReworkImplementationGap,
			summary:         "",
			overrideReason:  "",
			wantErr:         false,
			wantOutcomeType: OutcomeReworkImplementationGap,
		},
		{
			name:            "rework_implementation_gap with summary",
			results:         []string{ResultPass, ResultPass},
			outcome:         OutcomeReworkImplementationGap,
			summary:         "Need to rework implementation",
			overrideReason:  "",
			wantErr:         false,
			wantOutcomeType: OutcomeReworkImplementationGap,
		},
		{
			name:           "rework_implementation_gap no fail/unsure no summary",
			results:        []string{ResultPass, ResultPass},
			outcome:        OutcomeReworkImplementationGap,
			summary:        "",
			overrideReason: "",
			wantErr:        true,
			wantErrSubstr:  "rework_implementation_gap requires",
		},
		{
			name:            "rework_vision_change with summary",
			results:         []string{ResultPass, ResultPass},
			outcome:         OutcomeReworkVisionChange,
			summary:         "Vision needs to change",
			overrideReason:  "",
			wantErr:         false,
			wantOutcomeType: OutcomeReworkVisionChange,
		},
		{
			name:           "rework_vision_change no summary",
			results:        []string{ResultPass, ResultPass},
			outcome:        OutcomeReworkVisionChange,
			summary:        "",
			overrideReason: "",
			wantErr:        true,
			wantErrSubstr:  "rework_vision_change requires",
		},
		{
			name:          "invalid outcome",
			results:       []string{ResultPass},
			outcome:       "invalid_outcome",
			wantErr:       true,
			wantErrSubstr: "invalid outcome",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := reviewpacket.Outputs{
				ProductReview: reviewpacket.ProductReview{
					RunID: "run123",
				},
				ManualChecklist: reviewpacket.ManualChecklist{
					Items: makeCheckItems(len(tt.results)),
				},
			}

			session := Start(packet)
			for i, result := range tt.results {
				session.Checklist[i].Result = result
			}

			reviewOutcome, err := session.RecordOutcome(tt.outcome, tt.summary, tt.overrideReason)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RecordOutcome() = nil, want error")
				}
				if tt.wantErrSubstr != "" && err != nil {
					errStr := err.Error()
					if !contains(errStr, tt.wantErrSubstr) {
						t.Errorf("error = %q, want substring %q", errStr, tt.wantErrSubstr)
					}
				}
			} else {
				if err != nil {
					t.Errorf("RecordOutcome() = %v, want nil", err)
				}

				// Verify outcome was recorded
				if reviewOutcome == nil {
					t.Errorf("reviewOutcome = nil, want outcome")
				} else {
					if reviewOutcome.Outcome != tt.wantOutcomeType {
						t.Errorf("outcome = %q, want %q", reviewOutcome.Outcome, tt.wantOutcomeType)
					}
					if reviewOutcome.Summary != tt.summary {
						t.Errorf("summary = %q, want %q", reviewOutcome.Summary, tt.summary)
					}
					if reviewOutcome.OverrideReason != tt.overrideReason {
						t.Errorf("override reason = %q, want %q", reviewOutcome.OverrideReason, tt.overrideReason)
					}
					if reviewOutcome.RunID != "run123" {
						t.Errorf("RunID = %q, want %q", reviewOutcome.RunID, "run123")
					}
					if reviewOutcome.ReviewedAt.IsZero() {
						t.Errorf("ReviewedAt is zero")
					}
					if len(reviewOutcome.ManualResults) != len(tt.results) {
						t.Errorf("ManualResults length = %d, want %d", len(reviewOutcome.ManualResults), len(tt.results))
					}
					// Verify manual results match checklist
					for i, result := range reviewOutcome.ManualResults {
						if result.Result != tt.results[i] {
							t.Errorf("manual result %d = %q, want %q", i, result.Result, tt.results[i])
						}
					}
				}

				// Verify outcome was stored in session
				if session.Outcome != reviewOutcome {
					t.Errorf("session.Outcome not updated")
				}
			}
		})
	}
}

// Helper function to create checklist items
func makeCheckItems(count int) []reviewpacket.ManualCheckItem {
	items := make([]reviewpacket.ManualCheckItem, count)
	for i := 0; i < count; i++ {
		items[i] = reviewpacket.ManualCheckItem{
			ID:    "item" + fmt.Sprintf("%d", i),
			Title: "Item " + fmt.Sprintf("%d", i),
		}
	}
	return items
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestScenario_AcceptanceWithUnsureItemRequiresOverride verifies AC6:
// Acceptance with unsure items must require an override reason.
func TestScenario_AcceptanceWithUnsureItemRequiresOverride(t *testing.T) {
	packet := reviewpacket.Outputs{
		ProductReview: reviewpacket.ProductReview{
			RunID: "run123",
		},
		ManualChecklist: reviewpacket.ManualChecklist{
			Items: makeCheckItems(2),
		},
	}

	session := Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultUnsure

	// Test 1: Acceptance with unsure but no override should fail
	_, err := session.RecordOutcome(OutcomeAccepted, "", "")
	if err == nil {
		t.Errorf("RecordOutcome() expected error for unsure without override, got nil")
	}
	if !strings.Contains(err.Error(), "unsure items requires override reason") {
		t.Errorf("error = %q, want substring 'unsure items requires override reason'", err.Error())
	}

	// Test 2: Acceptance with unsure AND override should succeed
	outcome, err := session.RecordOutcome(OutcomeAccepted, "", "Reviewed manually")
	if err != nil {
		t.Errorf("RecordOutcome() with override = %v, want nil", err)
	}
	if outcome == nil {
		t.Errorf("outcome = nil, want outcome")
	} else if outcome.Outcome != OutcomeAccepted {
		t.Errorf("outcome.Outcome = %q, want %q", outcome.Outcome, OutcomeAccepted)
	}
}

// TestScenario_ReworkImplementationGapRequiresFlaggedItemOrSummary verifies AC7:
// Rework implementation gap requires either fail/unsure items or a non-empty summary.
func TestScenario_ReworkImplementationGapRequiresFlaggedItemOrSummary(t *testing.T) {
	packet := reviewpacket.Outputs{
		ProductReview: reviewpacket.ProductReview{
			RunID: "run123",
		},
		ManualChecklist: reviewpacket.ManualChecklist{
			Items: makeCheckItems(2),
		},
	}

	// Test 1: Rework with all pass and no summary should fail
	session := Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultPass

	_, err := session.RecordOutcome(OutcomeReworkImplementationGap, "", "")
	if err == nil {
		t.Errorf("RecordOutcome() expected error without fail/unsure/summary, got nil")
	}
	if !strings.Contains(err.Error(), "rework_implementation_gap requires") {
		t.Errorf("error = %q, want substring 'rework_implementation_gap requires'", err.Error())
	}

	// Test 2: Rework with fail item should succeed
	session = Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultFail

	outcome, err := session.RecordOutcome(OutcomeReworkImplementationGap, "", "")
	if err != nil {
		t.Errorf("RecordOutcome() with fail item = %v, want nil", err)
	}
	if outcome == nil || outcome.Outcome != OutcomeReworkImplementationGap {
		t.Errorf("outcome = %v, want rework_implementation_gap", outcome)
	}

	// Test 3: Rework with unsure item should succeed
	session = Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultUnsure

	outcome, err = session.RecordOutcome(OutcomeReworkImplementationGap, "", "")
	if err != nil {
		t.Errorf("RecordOutcome() with unsure item = %v, want nil", err)
	}
	if outcome == nil || outcome.Outcome != OutcomeReworkImplementationGap {
		t.Errorf("outcome = %v, want rework_implementation_gap", outcome)
	}

	// Test 4: Rework with all pass but summary should succeed
	session = Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultPass

	outcome, err = session.RecordOutcome(OutcomeReworkImplementationGap, "Need implementation rework", "")
	if err != nil {
		t.Errorf("RecordOutcome() with summary = %v, want nil", err)
	}
	if outcome == nil || outcome.Outcome != OutcomeReworkImplementationGap {
		t.Errorf("outcome = %v, want rework_implementation_gap", outcome)
	}
}

// TestScenario_ReworkVisionChangeRequiresSummary verifies AC8:
// Rework vision change requires a non-empty summary.
func TestScenario_ReworkVisionChangeRequiresSummary(t *testing.T) {
	packet := reviewpacket.Outputs{
		ProductReview: reviewpacket.ProductReview{
			RunID: "run123",
		},
		ManualChecklist: reviewpacket.ManualChecklist{
			Items: makeCheckItems(2),
		},
	}

	// Test 1: Rework vision change with empty summary should fail
	session := Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultPass

	_, err := session.RecordOutcome(OutcomeReworkVisionChange, "", "")
	if err == nil {
		t.Errorf("RecordOutcome() expected error with empty summary, got nil")
	}
	if !strings.Contains(err.Error(), "rework_vision_change requires") {
		t.Errorf("error = %q, want substring 'rework_vision_change requires'", err.Error())
	}

	// Test 2: Rework vision change with summary should succeed
	session = Start(packet)
	session.Checklist[0].Result = ResultPass
	session.Checklist[1].Result = ResultPass

	outcome, err := session.RecordOutcome(OutcomeReworkVisionChange, "Vision needs to change", "")
	if err != nil {
		t.Errorf("RecordOutcome() with summary = %v, want nil", err)
	}
	if outcome == nil {
		t.Errorf("outcome = nil, want outcome")
	} else if outcome.Outcome != OutcomeReworkVisionChange {
		t.Errorf("outcome.Outcome = %q, want %q", outcome.Outcome, OutcomeReworkVisionChange)
	} else if outcome.Summary != "Vision needs to change" {
		t.Errorf("outcome.Summary = %q, want %q", outcome.Summary, "Vision needs to change")
	}
}
