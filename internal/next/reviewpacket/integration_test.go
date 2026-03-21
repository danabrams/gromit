package reviewpacket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIntegration_CleanRun(t *testing.T) {
	// Scenario: ready_for_review run with scenarios produces all 5 artifacts with correct content
	// Given: a run reached ready_for_review with validation passed, acceptance criteria passed,
	// no blocking findings, and 2 scenarios in spec
	// When: the generator produces outputs and artifacts are written
	// Then: all 5 artifacts (validation, review, acceptance, product-review, process-review, manual-checklist)
	// exist with correct content; product-review has is_diagnostic: false, behavior cards with proven status,
	// no surprises; process-review has trust_level: "high"

	tempDir := t.TempDir()

	// Step 1: Write input artifacts (validation.json, review.json, acceptance.json)
	validationData := testValidationJSON(true, 8, 4)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	reviewData := map[string]interface{}{
		"diff_unavailable": false,
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"}, {"status": "pass"}, {"status": "pass"},
			{"status": "pass"}, {"status": "pass"},
		},
		"all_pass":            true,
		"has_fail_or_unclear": false,
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Step 2: Call generator to produce outputs
	gen := &Generator{}
	inputs := Inputs{
		RunID:         "test-run-clean",
		SpecTitle:     "Test Spec",
		SpecContent:   readyForReviewSpec(),
		TerminalState: "ready_for_review",
		ValidationResult: ValidationData{
			Passed: true,
			Checks: 12,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"info": {},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  5,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
	}

	outputs, err := gen.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	// Step 3: Write output artifacts
	outputs.ProductReview.NormalizeNilFields()
	if err := writeJSON(tempDir, "product-review.json", outputs.ProductReview); err != nil {
		t.Fatalf("write product-review.json: %v", err)
	}

	outputs.ProcessReview.NormalizeNilFields()
	if err := writeJSON(tempDir, "process-review.json", outputs.ProcessReview); err != nil {
		t.Fatalf("write process-review.json: %v", err)
	}

	outputs.ManualChecklist.NormalizeNilFields()
	if err := writeJSON(tempDir, "manual-checklist.json", outputs.ManualChecklist); err != nil {
		t.Fatalf("write manual-checklist.json: %v", err)
	}

	// Step 4: Verify all 5 artifacts exist
	requiredFiles := []string{
		"validation.json",
		"review.json",
		"acceptance.json",
		"product-review.json",
		"process-review.json",
		"manual-checklist.json",
	}

	for _, filename := range requiredFiles {
		filePath := filepath.Join(tempDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("artifact %s does not exist", filename)
		}
	}

	// Step 5: Verify product-review content
	productReviewPath := filepath.Join(tempDir, "product-review.json")
	data, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}

	var productReview ProductReview
	if err := json.Unmarshal(data, &productReview); err != nil {
		t.Fatalf("unmarshal product-review.json: %v", err)
	}

	if productReview.IsDiagnostic {
		t.Errorf("product-review.IsDiagnostic = true, want false")
	}
	if productReview.TerminalState != "ready_for_review" {
		t.Errorf("product-review.TerminalState = %q, want ready_for_review", productReview.TerminalState)
	}
	if len(productReview.BehaviorCards) != 2 {
		t.Errorf("len(BehaviorCards) = %d, want 2", len(productReview.BehaviorCards))
	}
	for i, card := range productReview.BehaviorCards {
		if card.AutomaticStatus != "proven" {
			t.Errorf("BehaviorCard[%d].AutomaticStatus = %q, want proven", i, card.AutomaticStatus)
		}
	}
	if len(productReview.Surprises) > 0 {
		t.Errorf("Surprises = %v, want empty", productReview.Surprises)
	}

	// Step 6: Verify process-review content
	processReviewPath := filepath.Join(tempDir, "process-review.json")
	data, err = os.ReadFile(processReviewPath)
	if err != nil {
		t.Fatalf("read process-review.json: %v", err)
	}

	var processReview ProcessReview
	if err := json.Unmarshal(data, &processReview); err != nil {
		t.Fatalf("unmarshal process-review.json: %v", err)
	}

	if processReview.TrustLevel != "high" {
		t.Errorf("process-review.TrustLevel = %q, want high", processReview.TrustLevel)
	}
	if processReview.RecommendedPosture != "quick_accept_path" {
		t.Errorf("process-review.RecommendedPosture = %q, want quick_accept_path", processReview.RecommendedPosture)
	}

	// Step 7: Verify manual-checklist content
	manualChecklistPath := filepath.Join(tempDir, "manual-checklist.json")
	data, err = os.ReadFile(manualChecklistPath)
	if err != nil {
		t.Fatalf("read manual-checklist.json: %v", err)
	}

	var manualChecklist ManualChecklist
	if err := json.Unmarshal(data, &manualChecklist); err != nil {
		t.Fatalf("unmarshal manual-checklist.json: %v", err)
	}

	if len(manualChecklist.Items) != 2 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 2", len(manualChecklist.Items))
	}
}

func TestIntegration_BlockedRun(t *testing.T) {
	// Scenario: blocked run produces diagnostic variant with correct markers
	// Given: a run ended in blocked state with validation failures
	// When: the generator produces outputs and artifacts are written
	// Then: product-review has is_diagnostic: true, blocker_summary and recommended_next_action populated,
	// all behavior cards have unclear status; process-review has trust_level: "low";
	// manual-checklist has empty items array

	tempDir := t.TempDir()

	// Write input artifacts
	validationData := testValidationJSON(false, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	reviewData := map[string]interface{}{
		"diff_unavailable": false,
		"blocking": []interface{}{
			map[string]interface{}{"message": "build failed"},
		},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results":            []map[string]interface{}{},
		"all_pass":           false,
		"has_fail_or_unclear": false,
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Generate outputs for blocked state
	gen := &Generator{}
	inputs := Inputs{
		RunID:         "test-blocked",
		SpecTitle:     "Test Spec",
		SpecContent:   readyForReviewSpec(),
		TerminalState: "blocked",
		ValidationResult: ValidationData{
			Passed: false,
			Checks: 5,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"blocking": {{Message: "build failed"}},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  0,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    3,
		RepeatedFailure: false,
	}

	outputs, err := gen.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	// Write output artifacts
	outputs.ProductReview.NormalizeNilFields()
	if err := writeJSON(tempDir, "product-review.json", outputs.ProductReview); err != nil {
		t.Fatalf("write product-review.json: %v", err)
	}

	outputs.ProcessReview.NormalizeNilFields()
	if err := writeJSON(tempDir, "process-review.json", outputs.ProcessReview); err != nil {
		t.Fatalf("write process-review.json: %v", err)
	}

	outputs.ManualChecklist.NormalizeNilFields()
	if err := writeJSON(tempDir, "manual-checklist.json", outputs.ManualChecklist); err != nil {
		t.Fatalf("write manual-checklist.json: %v", err)
	}

	// Verify all artifacts exist
	requiredFiles := []string{
		"validation.json",
		"review.json",
		"acceptance.json",
		"product-review.json",
		"process-review.json",
		"manual-checklist.json",
	}

	for _, filename := range requiredFiles {
		filePath := filepath.Join(tempDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("artifact %s does not exist", filename)
		}
	}

	// Verify product-review is diagnostic
	productReviewPath := filepath.Join(tempDir, "product-review.json")
	data, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}

	var productReview ProductReview
	if err := json.Unmarshal(data, &productReview); err != nil {
		t.Fatalf("unmarshal product-review.json: %v", err)
	}

	if !productReview.IsDiagnostic {
		t.Errorf("product-review.IsDiagnostic = false, want true")
	}
	if productReview.BlockerSummary == "" {
		t.Errorf("product-review.BlockerSummary is empty, want non-empty")
	}
	if productReview.RecommendedNextAction == "" {
		t.Errorf("product-review.RecommendedNextAction is empty, want non-empty")
	}

	for i, card := range productReview.BehaviorCards {
		if card.AutomaticStatus != "unclear" {
			t.Errorf("BehaviorCard[%d].AutomaticStatus = %q, want unclear", i, card.AutomaticStatus)
		}
	}

	// Verify process-review trust level
	processReviewPath := filepath.Join(tempDir, "process-review.json")
	data, err = os.ReadFile(processReviewPath)
	if err != nil {
		t.Fatalf("read process-review.json: %v", err)
	}

	var processReview ProcessReview
	if err := json.Unmarshal(data, &processReview); err != nil {
		t.Fatalf("unmarshal process-review.json: %v", err)
	}

	if processReview.TrustLevel != "low" {
		t.Errorf("process-review.TrustLevel = %q, want low", processReview.TrustLevel)
	}

	// Verify manual-checklist is empty for diagnostic
	manualChecklistPath := filepath.Join(tempDir, "manual-checklist.json")
	data, err = os.ReadFile(manualChecklistPath)
	if err != nil {
		t.Fatalf("read manual-checklist.json: %v", err)
	}

	var manualChecklist ManualChecklist
	if err := json.Unmarshal(data, &manualChecklist); err != nil {
		t.Fatalf("unmarshal manual-checklist.json: %v", err)
	}

	if len(manualChecklist.Items) != 0 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 0", len(manualChecklist.Items))
	}
}

func TestIntegration_NoScenariosFallback(t *testing.T) {
	// Scenario: spec with no scenarios falls back to acceptance criteria
	// Given: a spec has 3 acceptance criteria but no scenarios, run reached ready_for_review
	// When: the generator produces outputs and artifacts are written
	// Then: product-review contains 3 behavior cards (one per criterion);
	// manual-checklist has 3 items derived from the same criteria

	tempDir := t.TempDir()

	// Write input artifacts
	validationData := testValidationJSON(true, 6, 4)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	reviewData := map[string]interface{}{
		"diff_unavailable": false,
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"}, {"status": "pass"}, {"status": "pass"},
		},
		"all_pass":            true,
		"has_fail_or_unclear": false,
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Generate outputs with no-scenario spec
	gen := &Generator{}
	inputs := Inputs{
		RunID:         "test-no-scenarios",
		SpecTitle:     "Test Spec No Scenarios",
		SpecContent:   noScenarioSpec(),
		TerminalState: "ready_for_review",
		ValidationResult: ValidationData{
			Passed: true,
			Checks: 10,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"info": {},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  3,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
	}

	outputs, err := gen.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	// Write output artifacts
	outputs.ProductReview.NormalizeNilFields()
	if err := writeJSON(tempDir, "product-review.json", outputs.ProductReview); err != nil {
		t.Fatalf("write product-review.json: %v", err)
	}

	outputs.ProcessReview.NormalizeNilFields()
	if err := writeJSON(tempDir, "process-review.json", outputs.ProcessReview); err != nil {
		t.Fatalf("write process-review.json: %v", err)
	}

	outputs.ManualChecklist.NormalizeNilFields()
	if err := writeJSON(tempDir, "manual-checklist.json", outputs.ManualChecklist); err != nil {
		t.Fatalf("write manual-checklist.json: %v", err)
	}

	// Verify product-review has 3 behavior cards from criteria
	productReviewPath := filepath.Join(tempDir, "product-review.json")
	data, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}

	var productReview ProductReview
	if err := json.Unmarshal(data, &productReview); err != nil {
		t.Fatalf("unmarshal product-review.json: %v", err)
	}

	if len(productReview.BehaviorCards) != 3 {
		t.Errorf("len(BehaviorCards) = %d, want 3", len(productReview.BehaviorCards))
	}

	// Verify manual-checklist has 3 items from criteria
	manualChecklistPath := filepath.Join(tempDir, "manual-checklist.json")
	data, err = os.ReadFile(manualChecklistPath)
	if err != nil {
		t.Fatalf("read manual-checklist.json: %v", err)
	}

	var manualChecklist ManualChecklist
	if err := json.Unmarshal(data, &manualChecklist); err != nil {
		t.Fatalf("unmarshal manual-checklist.json: %v", err)
	}

	if len(manualChecklist.Items) != 3 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 3", len(manualChecklist.Items))
	}
}

// writeJSON is a helper that writes a value to a JSON file in the given directory.
func writeJSON(dir, filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), data, 0o644)
}
