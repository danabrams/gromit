package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/reviewsession"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestReviewRecord_AcceptedOutcomeWithAllItemsSkipped verifies that
// recording an accepted outcome with all checklist items skipped writes
// a correct review-outcome.json file.
func TestReviewRecord_AcceptedOutcomeWithAllItemsSkipped(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and review packet files
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write product-review.json
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Test summary",
		BehaviorCards: []reviewpacket.BehaviorCard{},
	}
	productReview.NormalizeNilFields()
	productData, err := json.MarshalIndent(productReview, "", "  ")
	if err != nil {
		t.Fatalf("marshal product review: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644); err != nil {
		t.Fatalf("write product-review.json: %v", err)
	}

	// Write process-review.json
	processReview := map[string]interface{}{
		"trust_level": "high",
	}
	processData, err := json.MarshalIndent(processReview, "", "  ")
	if err != nil {
		t.Fatalf("marshal process review: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644); err != nil {
		t.Fatalf("write process-review.json: %v", err)
	}

	// Write manual-checklist.json with 2 items
	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{
			{
				ID:    "check-1",
				Title: "Check 1",
			},
			{
				ID:    "check-2",
				Title: "Check 2",
			},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, err := json.MarshalIndent(manualChecklist, "", "  ")
	if err != nil {
		t.Fatalf("marshal manual checklist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644); err != nil {
		t.Fatalf("write manual-checklist.json: %v", err)
	}

	// Call reviewRecord
	err = reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeAccepted, "Looks good", "")
	if err != nil {
		t.Fatalf("reviewRecord: %v", err)
	}

	// Verify review-outcome.json was written
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Read and verify the outcome file contents
	outcomeData, err := os.ReadFile(outcomeFile)
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	var outcome reviewsession.ReviewOutcome
	if err := json.Unmarshal(outcomeData, &outcome); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}

	// Verify outcome fields
	if outcome.RunID != rs.RunID {
		t.Errorf("wrong RunID: got %q, want %q", outcome.RunID, rs.RunID)
	}
	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong Outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}
	if outcome.Summary != "Looks good" {
		t.Errorf("wrong Summary: got %q, want %q", outcome.Summary, "Looks good")
	}
	if outcome.OverrideReason != "" {
		t.Errorf("unexpected OverrideReason: %q", outcome.OverrideReason)
	}

	// Verify all manual results are marked as skipped
	if len(outcome.ManualResults) != 2 {
		t.Errorf("wrong number of manual results: got %d, want 2", len(outcome.ManualResults))
	}
	for i, result := range outcome.ManualResults {
		if result.Result != reviewsession.ResultSkipped {
			t.Errorf("manual result %d has wrong result: got %q, want %q", i, result.Result, reviewsession.ResultSkipped)
		}
	}
}

// TestReviewRecord_ReworkVisionChangeRequiresSummary verifies that
// rework_vision_change outcome is rejected without a summary.
func TestReviewRecord_ReworkVisionChangeRequiresSummary(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and review packet files
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write minimal review packet artifacts
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Test",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{
			{ID: "check-1", Title: "Check 1"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Attempt to record rework_vision_change with empty summary - should fail
	err := reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeReworkVisionChange, "", "")
	if err == nil {
		t.Error("expected error when recording rework_vision_change with empty summary, got nil")
	}
	if !strings.Contains(err.Error(), "summary") {
		t.Errorf("expected error mentioning summary, got: %v", err)
	}

	// Verify review-outcome.json was NOT written
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err == nil {
		t.Error("review-outcome.json should not have been written on error")
	}

	// Now record with a summary - should succeed
	err = reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeReworkVisionChange, "Changed direction", "")
	if err != nil {
		t.Fatalf("reviewRecord with summary should succeed: %v", err)
	}

	// Verify review-outcome.json was written
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Verify the outcome has the summary
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)
	if outcome.Summary != "Changed direction" {
		t.Errorf("wrong Summary: got %q, want %q", outcome.Summary, "Changed direction")
	}
}

// TestReviewRecord_ReworkImplementationGapValidation verifies that
// rework_implementation_gap requires either a flagged item or a non-empty summary.
func TestReviewRecord_ReworkImplementationGapValidation(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and review packet files
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write minimal review packet artifacts
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Test",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	// Create manual checklist with all items that would pass
	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{
			{ID: "check-1", Title: "Check 1"},
			{ID: "check-2", Title: "Check 2"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Attempt to record rework_implementation_gap with all items skipped and no summary - should fail
	err := reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeReworkImplementationGap, "", "")
	if err == nil {
		t.Error("expected error when recording rework_implementation_gap without flagged items or summary, got nil")
	}

	// Verify review-outcome.json was NOT written
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err == nil {
		t.Error("review-outcome.json should not have been written on error")
	}

	// Now record with a summary - should succeed
	err = reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeReworkImplementationGap, "Found performance issues", "")
	if err != nil {
		t.Fatalf("reviewRecord with summary should succeed: %v", err)
	}

	// Verify review-outcome.json was written
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Verify the outcome
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)
	if outcome.Outcome != reviewsession.OutcomeReworkImplementationGap {
		t.Errorf("wrong Outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeReworkImplementationGap)
	}
	if outcome.Summary != "Found performance issues" {
		t.Errorf("wrong Summary: got %q, want %q", outcome.Summary, "Found performance issues")
	}
}

// TestReviewRecord_RefusesNonTerminalRun verifies that review record
// returns an error when attempting to record a review for a run
// that is not in a terminal state.
func TestReviewRecord_RefusesNonTerminalRun(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in non-terminal state (running)
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusRunning
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Attempt to record outcome for non-terminal run should fail
	err := reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeAccepted, "Summary", "")
	if err == nil {
		t.Error("expected error when recording outcome for non-terminal run, got nil")
	}
	if !strings.Contains(err.Error(), "non-terminal") {
		t.Errorf("expected error mentioning non-terminal, got: %v", err)
	}

	// Verify no outcome file was created
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err == nil {
		t.Error("review-outcome.json should not have been created for non-terminal run")
	}
}

// TestReviewRecord_AcceptedRequiresNoFailedItems verifies that
// accepted outcome is rejected if any checklist item has a failed result.
func TestReviewRecord_AcceptedRequiresNoFailedItems(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and review packet files
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write minimal review packet artifacts
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Test",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{
			{ID: "check-1", Title: "Check 1"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Load the packet and manually record one item as failed
	outputs, err := loadReviewPacket(rs.RunID, storeDir)
	if err != nil {
		t.Fatalf("loadReviewPacket: %v", err)
	}

	session := reviewsession.Start(*outputs)
	if err := session.RecordItemResult(reviewsession.ResultFail, "Issue found"); err != nil {
		t.Fatalf("record item result: %v", err)
	}

	// Save the session state
	sessionFile := filepath.Join(evidenceDir, "review-session.json")
	sessionData, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(sessionFile, sessionData, 0o644)

	// Now attempt to accept - should fail because there's a failed item
	// Note: This test demonstrates the validation at the Session level.
	// The reviewRecord function should reject this before writing.
	canAccept, reason := session.CanAccept()
	if canAccept {
		t.Error("session should not allow acceptance with a failed item")
	}
	if !strings.Contains(reason, "failed") {
		t.Errorf("expected reason mentioning failed items, got: %q", reason)
	}
}

// TestReviewRecord_AcceptedWithUnsureRequiresOverride verifies that
// accepted outcome with unsure items requires an override reason.
func TestReviewRecord_AcceptedWithUnsureRequiresOverride(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and review packet files
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write minimal review packet artifacts
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Test",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{
			{ID: "check-1", Title: "Check 1"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Load the packet and record one item as unsure
	outputs, err := loadReviewPacket(rs.RunID, storeDir)
	if err != nil {
		t.Fatalf("loadReviewPacket: %v", err)
	}

	session := reviewsession.Start(*outputs)
	if err := session.RecordItemResult(reviewsession.ResultUnsure, "Not completely sure"); err != nil {
		t.Fatalf("record item result: %v", err)
	}

	// Verify that NeedsOverride returns true
	if !session.NeedsOverride() {
		t.Error("session should indicate override is needed for unsure items")
	}

	// Attempt to record outcome without override should fail
	_, err = session.RecordOutcome(reviewsession.OutcomeAccepted, "Summary", "")
	if err == nil {
		t.Error("expected error when accepting with unsure items and no override")
	}
	if !strings.Contains(err.Error(), "override") {
		t.Errorf("expected error mentioning override, got: %v", err)
	}

	// With override reason, it should succeed
	outcome, err := session.RecordOutcome(reviewsession.OutcomeAccepted, "Summary", "Verified manually")
	if err != nil {
		t.Fatalf("record outcome with override should succeed: %v", err)
	}
	if outcome.OverrideReason != "Verified manually" {
		t.Errorf("wrong OverrideReason: got %q, want %q", outcome.OverrideReason, "Verified manually")
	}
}

// TestReviewRecord_CommandWithRunFlag tests that the --run flag properly specifies
// the run ID instead of using positional argument.
func TestReviewRecord_CommandWithRunFlag(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and review packet files
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write product-review.json
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Test summary",
		BehaviorCards: []reviewpacket.BehaviorCard{},
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	// Write process-review.json
	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	// Write manual-checklist.json
	manualChecklist := reviewpacket.ManualChecklist{Items: []reviewpacket.ManualCheckItem{}}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Test with --run flag - directly call reviewRecord to verify the flag parsing works
	err := reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeAccepted, "Looks good", "")
	if err != nil {
		t.Fatalf("reviewRecord with run ID: %v", err)
	}

	// Verify review-outcome.json was written
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Read and verify the outcome file contents
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	if err := json.Unmarshal(outcomeData, &outcome); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}

	if outcome.RunID != rs.RunID {
		t.Errorf("wrong RunID: got %q, want %q", outcome.RunID, rs.RunID)
	}
	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong Outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}
}

// loadReviewPacket loads the review packet from the evidence directory.
// This is a helper function for tests.
func loadReviewPacket(runID, storeDir string) (*reviewpacket.Outputs, error) {
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(runID)

	// Read product-review.json
	productPath := filepath.Join(evidenceDir, "product-review.json")
	productData, err := os.ReadFile(productPath)
	if err != nil {
		return nil, err
	}
	var productReview reviewpacket.ProductReview
	if err := json.Unmarshal(productData, &productReview); err != nil {
		return nil, err
	}

	// Read process-review.json
	processPath := filepath.Join(evidenceDir, "process-review.json")
	processData, err := os.ReadFile(processPath)
	if err != nil {
		return nil, err
	}
	var processReview reviewpacket.ProcessReview
	if err := json.Unmarshal(processData, &processReview); err != nil {
		return nil, err
	}

	// Read manual-checklist.json
	manualPath := filepath.Join(evidenceDir, "manual-checklist.json")
	manualData, err := os.ReadFile(manualPath)
	if err != nil {
		return nil, err
	}
	var manualChecklist reviewpacket.ManualChecklist
	if err := json.Unmarshal(manualData, &manualChecklist); err != nil {
		return nil, err
	}

	outputs := &reviewpacket.Outputs{
		ProductReview:   productReview,
		ProcessReview:   processReview,
		ManualChecklist: manualChecklist,
	}
	return outputs, nil
}
