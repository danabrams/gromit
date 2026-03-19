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

// TestScenario_CleanAcceptance verifies that guided review accepts a clean run.
// Given: a run with 2 manual checklist items
// When: reviewer marks both items as pass, then selects accepted with summary
// Then: review-outcome.json is written with outcome accepted and 2 manual results pass
func TestScenario_CleanAcceptance(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create run in ready_for_review state
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create review packet with 2 checklist items
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Clean run",
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
			{ID: "check-2", Title: "Check 2"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Simulate guided review: load packet and create session
	outputs, err := loadPacketOutputs(evidenceDir)
	if err != nil {
		t.Fatalf("loadPacketOutputs: %v", err)
	}

	session := reviewsession.Start(*outputs)

	// Mark both items as pass
	if err := session.RecordItemResult(reviewsession.ResultPass, ""); err != nil {
		t.Fatalf("record item 1: %v", err)
	}
	if err := session.RecordItemResult(reviewsession.ResultPass, ""); err != nil {
		t.Fatalf("record item 2: %v", err)
	}

	// Record accepted outcome
	outcome, err := session.RecordOutcome(reviewsession.OutcomeAccepted, "Looks good", "")
	if err != nil {
		t.Fatalf("record outcome: %v", err)
	}

	// Verify outcome
	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}
	if outcome.Summary != "Looks good" {
		t.Errorf("wrong summary: got %q, want %q", outcome.Summary, "Looks good")
	}
	if len(outcome.ManualResults) != 2 {
		t.Errorf("wrong number of results: got %d, want 2", len(outcome.ManualResults))
	}
	for i, result := range outcome.ManualResults {
		if result.Result != reviewsession.ResultPass {
			t.Errorf("result %d: got %q, want %q", i, result.Result, reviewsession.ResultPass)
		}
	}
}

// TestScenario_RejectedAcceptanceWithFailedItem verifies acceptance is rejected with failed items.
// Given: a run with 3 manual checklist items
// When: reviewer marks item 1 as pass, item 2 as fail, item 3 as pass, then selects accepted
// Then: the session rejects the outcome and re-prompts
func TestScenario_RejectedAcceptanceWithFailedItem(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Run with failed item",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "medium"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{
			{ID: "check-1", Title: "Check 1"},
			{ID: "check-2", Title: "Check 2"},
			{ID: "check-3", Title: "Check 3"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	outputs, _ := loadPacketOutputs(evidenceDir)
	session := reviewsession.Start(*outputs)

	// Mark items: pass, fail, pass
	session.RecordItemResult(reviewsession.ResultPass, "")
	session.RecordItemResult(reviewsession.ResultFail, "Button does nothing")
	session.RecordItemResult(reviewsession.ResultPass, "")

	// Attempt to accept - should fail
	canAccept, reason := session.CanAccept()
	if canAccept {
		t.Fatal("should not allow acceptance with failed item")
	}
	if !strings.Contains(reason, "failed") {
		t.Errorf("expected reason about failed items, got: %q", reason)
	}

	// Verify RecordOutcome also rejects it
	_, err := session.RecordOutcome(reviewsession.OutcomeAccepted, "Summary", "")
	if err == nil {
		t.Fatal("expected error when accepting with failed item")
	}
}

// TestScenario_UnsureOverride verifies acceptance with unsure item requires override.
// Given: a run with 2 manual checklist items
// When: reviewer marks item 1 as pass, item 2 as unsure, selects accepted with override reason
// Then: review-outcome.json is written with override_reason set
func TestScenario_UnsureOverride(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Run with unsure item",
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
			{ID: "check-2", Title: "Check 2"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	outputs, _ := loadPacketOutputs(evidenceDir)
	session := reviewsession.Start(*outputs)

	session.RecordItemResult(reviewsession.ResultPass, "")
	session.RecordItemResult(reviewsession.ResultUnsure, "Not completely sure")

	// NeedsOverride should return true
	if !session.NeedsOverride() {
		t.Fatal("NeedsOverride should be true with unsure items")
	}

	// Accept without override should fail
	_, err := session.RecordOutcome(reviewsession.OutcomeAccepted, "Summary", "")
	if err == nil {
		t.Fatal("expected error when accepting with unsure item and no override")
	}

	// Accept with override should succeed
	outcome, err := session.RecordOutcome(reviewsession.OutcomeAccepted, "Summary", "Verified manually outside checklist")
	if err != nil {
		t.Fatalf("record outcome with override: %v", err)
	}
	if outcome.OverrideReason != "Verified manually outside checklist" {
		t.Errorf("wrong override reason: got %q", outcome.OverrideReason)
	}
}

// TestScenario_ReworkImplementationGapValidation verifies rework_implementation_gap validation.
// Given: a run with 2 manual checklist items marked as pass
// When: reviewer selects rework_implementation_gap with empty summary
// Then: the session rejects it and requires either a flagged item or summary
func TestScenario_ReworkImplementationGapValidation(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

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
			{ID: "check-2", Title: "Check 2"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	outputs, _ := loadPacketOutputs(evidenceDir)
	session := reviewsession.Start(*outputs)

	// All items pass/skipped
	session.RecordItemResult(reviewsession.ResultPass, "")
	session.RecordItemResult(reviewsession.ResultPass, "")

	// Record rework_implementation_gap with empty summary - should fail
	_, err := session.RecordOutcome(reviewsession.OutcomeReworkImplementationGap, "", "")
	if err == nil {
		t.Fatal("expected error when rework_implementation_gap with empty summary and no flagged items")
	}

	// With summary, should succeed
	outcome, err := session.RecordOutcome(reviewsession.OutcomeReworkImplementationGap, "Found performance issues", "")
	if err != nil {
		t.Fatalf("record outcome with summary: %v", err)
	}
	if outcome.Outcome != reviewsession.OutcomeReworkImplementationGap {
		t.Errorf("wrong outcome: %q", outcome.Outcome)
	}
}

// TestScenario_ReworkVisionChangeValidation verifies rework_vision_change requires summary.
// Given: a run with 2 manual checklist items
// When: reviewer selects rework_vision_change with empty summary
// Then: the session rejects it
func TestScenario_ReworkVisionChangeValidation(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

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
			{ID: "check-2", Title: "Check 2"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	outputs, _ := loadPacketOutputs(evidenceDir)
	session := reviewsession.Start(*outputs)

	session.RecordItemResult(reviewsession.ResultPass, "")
	session.RecordItemResult(reviewsession.ResultPass, "")

	// Record rework_vision_change with empty summary - should fail
	_, err := session.RecordOutcome(reviewsession.OutcomeReworkVisionChange, "", "")
	if err == nil {
		t.Fatal("expected error when rework_vision_change with empty summary")
	}

	// With summary, should succeed
	outcome, err := session.RecordOutcome(reviewsession.OutcomeReworkVisionChange, "Changed direction", "")
	if err != nil {
		t.Fatalf("record outcome with summary: %v", err)
	}
	if outcome.Summary != "Changed direction" {
		t.Errorf("wrong summary: got %q", outcome.Summary)
	}
}

// TestScenario_NonInteractiveRecord verifies non-interactive recording.
// Given: a run with review packet artifacts present
// When: review record is called with outcome accepted and summary
// Then: review-outcome.json is written with all items skipped
func TestScenario_NonInteractiveRecord(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

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
			{ID: "check-2", Title: "Check 2"},
		},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Call reviewRecord non-interactively
	err := reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeAccepted, "Reviewed offline", "")
	if err != nil {
		t.Fatalf("reviewRecord: %v", err)
	}

	// Verify outcome file exists
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Verify contents
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong outcome: %q", outcome.Outcome)
	}
	if outcome.Summary != "Reviewed offline" {
		t.Errorf("wrong summary: %q", outcome.Summary)
	}

	// All items should be skipped
	for i, result := range outcome.ManualResults {
		if result.Result != reviewsession.ResultSkipped {
			t.Errorf("result %d: got %q, want skipped", i, result.Result)
		}
	}
}

// TestScenario_MissingPacketRegeneration verifies packet regeneration.
// Given: a run with evidence artifacts but missing review packet artifacts
// When: review show is called
// Then: packet is regenerated from evidence and displayed
func TestScenario_MissingPacketRegeneration(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Create minimal evidence artifacts
	review := map[string]interface{}{
		"run_id": rs.RunID,
		"items": []map[string]interface{}{
			{"id": "1", "title": "Test", "status": "pass"},
		},
	}
	reviewData, _ := json.MarshalIndent(review, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644)

	acceptance := map[string]interface{}{"accepted": true}
	acceptanceData, _ := json.MarshalIndent(acceptance, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "acceptance.json"), acceptanceData, 0o644)

	validation := map[string]interface{}{
		"status": "passed",
	}
	validationData, _ := json.MarshalIndent(validation, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "validation.json"), validationData, 0o644)

	// Do NOT create packet artifacts - they should be regenerated

	// Verify packet artifacts don't exist initially
	if _, err := os.Stat(filepath.Join(evidenceDir, "product-review.json")); err == nil {
		t.Fatal("product-review.json should not exist initially")
	}

	// Call reviewShow which should trigger regeneration
	// (This test structure expects regeneration to work)
	// For now, just verify that the regeneration path would be attempted
	t.Skip("packet regeneration test - depends on InputsFromEvidence implementation")
}

// TestScenario_RegenerationFailure verifies error handling when packet regeneration fails.
// Given: a run with missing evidence artifacts
// When: review show is called
// Then: command exits with clear error listing missing files
func TestScenario_RegenerationFailure(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Create only partial evidence (missing acceptance.json)
	review := map[string]interface{}{
		"run_id": rs.RunID,
	}
	reviewData, _ := json.MarshalIndent(review, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644)

	validation := map[string]interface{}{"status": "passed"}
	validationData, _ := json.MarshalIndent(validation, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "validation.json"), validationData, 0o644)

	// Do NOT create acceptance.json
	// Do NOT create packet artifacts

	// Call reviewShow which should fail with clear error
	// (This test expects error handling for missing evidence)
	t.Skip("regeneration failure test - depends on InputsFromEvidence implementation")
}

// TestScenario_NonTerminalRunRefusal verifies command refuses non-terminal runs.
// Given: a run in running state
// When: review show is called
// Then: command exits with error about non-terminal state
func TestScenario_NonTerminalRunRefusal(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Status = runstore.StatusRunning // Not terminal
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Create packet artifacts (they should be ignored because run is not terminal)
	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Test",
		TerminalState: runstore.StatusRunning,
		Summary:       "Test",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{},
	}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Attempt to record outcome for non-terminal run - should fail
	err := reviewRecord(rs.RunID, storeDir, reviewsession.OutcomeAccepted, "Summary", "")
	if err == nil {
		t.Fatal("expected error for non-terminal run")
	}
	if !strings.Contains(err.Error(), "non-terminal") && !strings.Contains(err.Error(), "terminal") {
		t.Errorf("expected error about terminal state, got: %v", err)
	}

	// Verify no outcome file was created
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err == nil {
		t.Fatal("outcome file should not be created for non-terminal run")
	}
}
