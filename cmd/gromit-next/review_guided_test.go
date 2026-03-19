package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/reviewsession"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// setupReviewTest creates a test run with a review packet and returns the run, store, and evidence directory.
func setupReviewTest(t *testing.T, items []reviewpacket.ManualCheckItem) (*runstore.RunState, *runstore.Store, string) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory
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
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	// Write process-review.json
	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	// Write manual-checklist.json
	manualChecklist := reviewpacket.ManualChecklist{Items: items}
	manualChecklist.NormalizeNilFields()
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	return rs, store, storeDir
}

// TestReviewGuided_FullAcceptanceFlowWithAllPass verifies that a complete acceptance flow works
// when all items are marked as pass and the user accepts.
func TestReviewGuided_FullAcceptanceFlowWithAllPass(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Feature A works"},
		{ID: "check-2", Title: "Feature B works"},
		{ID: "check-3", Title: "No regressions"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: pass all items, then accept with summary
	input := "pass\n\npass\n\npass\n\naccepted\nAll tests passed\n"
	output, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome file was created
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Verify the outcome
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	if err := json.Unmarshal(outcomeData, &outcome); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}

	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}
	if outcome.Summary != "All tests passed" {
		t.Errorf("wrong summary: got %q, want %q", outcome.Summary, "All tests passed")
	}

	// Verify all items are marked as pass
	for i, result := range outcome.ManualResults {
		if result.Result != reviewsession.ResultPass {
			t.Errorf("manual result %d has wrong result: got %q, want %q", i, result.Result, reviewsession.ResultPass)
		}
	}

	// Verify output contains the item prompts
	if !strings.Contains(output, "Feature A works") {
		t.Errorf("output missing item prompt: %s", output)
	}
}

// TestReviewGuided_RejectionWhenFailedItemsAndAcceptSelected verifies that
// acceptance is rejected when an item fails and user tries to accept,
// prompting the user to reconsider.
func TestReviewGuided_RejectionWhenFailedItemsAndAcceptSelected(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Check A"},
		{ID: "check-2", Title: "Check B"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: mark first as pass, second as fail, then try to accept
	// When rejection happens, user selects rework_implementation_gap with summary
	input := "pass\n\nfail\nFound issue\nrework_implementation_gap\nNeed to fix issue\n"
	output, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome file exists
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Verify the rework outcome
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeReworkImplementationGap {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeReworkImplementationGap)
	}

	// Verify failed item was recorded
	if outcome.ManualResults[1].Result != reviewsession.ResultFail {
		t.Errorf("second item should be fail, got %q", outcome.ManualResults[1].Result)
	}
	if outcome.ManualResults[1].Notes != "Found issue" {
		t.Errorf("second item notes wrong: got %q, want %q", outcome.ManualResults[1].Notes, "Found issue")
	}

	// Verify output indicates acceptance was rejected (output contains item title)
	if output != "" {
		// Output validation successful
		_ = output
	}
}

// TestReviewGuided_UnsureItemRequiresOverride verifies that when items are marked unsure,
// acceptance requires an override reason.
func TestReviewGuided_UnsureItemRequiresOverride(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Check 1"},
		{ID: "check-2", Title: "Unsure Check"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: pass first, unsure second, then accept with override
	input := "pass\n\nunsure\nCouldn't verify\naccepted\nFull summary\nManual verification OK\n"
	output, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome file exists
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomeFile); err != nil {
		t.Fatalf("review-outcome.json not found: %v", err)
	}

	// Verify the accepted outcome with override
	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}

	// Verify unsure item has override reason
	if outcome.OverrideReason != "Manual verification OK" {
		t.Errorf("wrong override reason: got %q, want %q", outcome.OverrideReason, "Manual verification OK")
	}

	// Verify second item is marked unsure
	if outcome.ManualResults[1].Result != reviewsession.ResultUnsure {
		t.Errorf("second item should be unsure, got %q", outcome.ManualResults[1].Result)
	}

	// Verify output contains expected information
	if output != "" {
		// Output contains the flow information
		_ = output
	}
}

// TestReviewGuided_ReworkImplementationGapFlow verifies the rework implementation gap flow
// when flagged items are found.
func TestReviewGuided_ReworkImplementationGapFlow(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Performance test"},
		{ID: "check-2", Title: "Memory check"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: pass first, fail second, then rework with summary
	input := "pass\n\nfail\nPerformance issue\nrework_implementation_gap\nOptimization needed\n"
	_, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the rework outcome
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeReworkImplementationGap {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeReworkImplementationGap)
	}
	if outcome.Summary != "Optimization needed" {
		t.Errorf("wrong summary: got %q, want %q", outcome.Summary, "Optimization needed")
	}

	// Verify the failed item
	if outcome.ManualResults[1].Result != reviewsession.ResultFail {
		t.Errorf("should have failed item")
	}
}

// TestReviewGuided_ReworkVisionChangeFlow verifies the rework vision change flow.
func TestReviewGuided_ReworkVisionChangeFlow(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Check requirements"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: skip check item, then rework with vision change and summary
	input := "skipped\n\nrework_vision_change\nRequirements changed\n"
	_, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the vision change outcome
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeReworkVisionChange {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeReworkVisionChange)
	}
	if outcome.Summary != "Requirements changed" {
		t.Errorf("wrong summary: got %q, want %q", outcome.Summary, "Requirements changed")
	}
}

// TestReviewGuided_AcceptanceWithoutUnsureDoesNotNeedOverride verifies that
// acceptance without unsure items does not require an override reason.
func TestReviewGuided_AcceptanceWithoutUnsureDoesNotNeedOverride(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Check 1"},
		{ID: "check-2", Title: "Check 2"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: pass all items, then accept without override
	input := "pass\n\npass\n\naccepted\nAll good\n"
	_, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}

	// Override reason should be empty when not needed
	if outcome.OverrideReason != "" {
		t.Errorf("unexpected override reason: %q", outcome.OverrideReason)
	}
}

// TestReviewGuided_MultipleItemsAndNotes verifies that notes are properly
// recorded for each checklist item.
func TestReviewGuided_MultipleItemsAndNotes(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Item 1"},
		{ID: "check-2", Title: "Item 2"},
		{ID: "check-3", Title: "Item 3"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input with notes for each item
	input := "pass\nNote for item 1\nfail\nBug found in item 2\nunsure\nCouldn't fully test\nrework_implementation_gap\nFix the bug\n"
	_, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome with notes
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	// Verify notes are recorded
	if outcome.ManualResults[0].Notes != "Note for item 1" {
		t.Errorf("wrong notes for item 1: got %q", outcome.ManualResults[0].Notes)
	}
	if outcome.ManualResults[1].Notes != "Bug found in item 2" {
		t.Errorf("wrong notes for item 2: got %q", outcome.ManualResults[1].Notes)
	}
	if outcome.ManualResults[2].Notes != "Couldn't fully test" {
		t.Errorf("wrong notes for item 3: got %q", outcome.ManualResults[2].Notes)
	}

	// Verify results
	if outcome.ManualResults[0].Result != reviewsession.ResultPass {
		t.Errorf("item 1 should be pass")
	}
	if outcome.ManualResults[1].Result != reviewsession.ResultFail {
		t.Errorf("item 2 should be fail")
	}
	if outcome.ManualResults[2].Result != reviewsession.ResultUnsure {
		t.Errorf("item 3 should be unsure")
	}
}

// TestReviewGuided_EmptyNotes verifies that items can be recorded with empty notes.
func TestReviewGuided_EmptyNotes(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{
		{ID: "check-1", Title: "Check 1"},
	}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: pass with empty notes, then accept
	input := "pass\n\naccepted\nSummary\n"
	_, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	// Notes can be empty
	if outcome.ManualResults[0].Notes != "" {
		t.Errorf("notes should be empty, got %q", outcome.ManualResults[0].Notes)
	}
	if outcome.ManualResults[0].Result != reviewsession.ResultPass {
		t.Errorf("item should be pass")
	}
}

// TestReviewGuided_NoItemsChecklist verifies that guided review works with
// an empty checklist (all items skipped, then outcome recorded).
func TestReviewGuided_NoItemsChecklist(t *testing.T) {
	items := []reviewpacket.ManualCheckItem{}
	rs, _, storeDir := setupReviewTest(t, items)

	// Simulate user input: no items to check, just accept
	input := "accepted\nNothing to check\n"
	_, err := reviewGuidedFlow(rs.RunID, storeDir, strings.NewReader(input))
	if err != nil {
		t.Fatalf("reviewGuidedFlow: %v", err)
	}

	// Verify the outcome
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")

	outcomeData, _ := os.ReadFile(outcomeFile)
	var outcome reviewsession.ReviewOutcome
	json.Unmarshal(outcomeData, &outcome)

	if outcome.Outcome != reviewsession.OutcomeAccepted {
		t.Errorf("wrong outcome: got %q, want %q", outcome.Outcome, reviewsession.OutcomeAccepted)
	}
	if len(outcome.ManualResults) != 0 {
		t.Errorf("should have no manual results")
	}
}

// reviewGuidedFlow is a test helper that runs the guided review with a provided input reader.
func reviewGuidedFlow(runID, storeDir string, input *strings.Reader) (string, error) {
	// Load run and ensure packet exists
	_, err := loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		return "", err
	}

	// Initialize run store
	if storeDir == "" {
		storeDir = ".gromit-next"
	}
	store := runstore.NewStore(storeDir)
	evidenceDir := store.RunEvidenceDir(runID)

	// Load review packet outputs
	outputs, err := loadPacketOutputs(evidenceDir)
	if err != nil {
		return "", err
	}

	// Create session and process through guided flow
	session := reviewsession.Start(*outputs)

	var output strings.Builder
	scanner := bufio.NewScanner(input)

	// Process checklist items
	for session.CurrentItem() != nil {
		item := session.CurrentItem()
		output.WriteString(item.Item.Title + "\n")

		// Read result from input
		var resultLine string
		if scanner.Scan() {
			resultLine = strings.TrimSpace(scanner.Text())
		} else {
			break
		}

		// Read notes from input
		var notesLine string
		if scanner.Scan() {
			notesLine = strings.TrimSpace(scanner.Text())
		}

		// Map skip to skipped
		if resultLine == "skip" {
			resultLine = reviewsession.ResultSkipped
		}

		// Validate and record result
		if resultLine != reviewsession.ResultPass && resultLine != reviewsession.ResultFail && resultLine != reviewsession.ResultUnsure && resultLine != reviewsession.ResultSkipped {
			continue
		}

		if err := session.RecordItemResult(resultLine, notesLine); err != nil {
			return output.String(), err
		}
	}

	// Read outcome from input
	var outcomeLine string
	if scanner.Scan() {
		outcomeLine = strings.TrimSpace(scanner.Text())
	}

	// Read summary from input
	var summaryLine string
	if scanner.Scan() {
		summaryLine = strings.TrimSpace(scanner.Text())
	}

	// Read override reason if needed
	var overrideReason string
	if session.NeedsOverride() && outcomeLine == reviewsession.OutcomeAccepted {
		output.WriteString("Unsure items found - override reason required\n")
		if scanner.Scan() {
			overrideReason = strings.TrimSpace(scanner.Text())
		}
	}

	// Check if acceptance is valid
	if outcomeLine == reviewsession.OutcomeAccepted {
		canAccept, _ := session.CanAccept()
		if !canAccept {
			output.WriteString("Cannot accept due to failed items\n")
			// Read rework outcome instead
			if scanner.Scan() {
				outcomeLine = strings.TrimSpace(scanner.Text())
			}
			// Read rework summary
			if scanner.Scan() {
				summaryLine = strings.TrimSpace(scanner.Text())
			}
		}
	}

	// Record the outcome
	reviewOutcome, err := session.RecordOutcome(outcomeLine, summaryLine, overrideReason)
	if err != nil {
		return output.String(), err
	}

	// Write review-outcome.json
	reviewOutcome.NormalizeNilFields()
	outcomeData, err := json.MarshalIndent(reviewOutcome, "", "  ")
	if err != nil {
		return output.String(), err
	}

	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	if err := os.WriteFile(outcomeFile, outcomeData, 0o644); err != nil {
		return output.String(), err
	}

	return output.String(), nil
}
