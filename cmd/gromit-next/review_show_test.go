package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestReviewShow_PrintsProductReviewAndTrustBanner verifies that review show
// outputs product review markdown followed by a trust level banner for a terminal run.
func TestReviewShow_PrintsProductReviewAndTrustBanner(t *testing.T) {
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
		BehaviorCards: []reviewpacket.BehaviorCard{
			{
				ID:              "behavior-1",
				Title:           "Feature A works",
				AutomaticStatus: "passed",
			},
		},
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

	// Write manual-checklist.json (required by loadRunAndEnsurePacket)
	manualChecklist := map[string]interface{}{
		"items": []interface{}{},
	}
	manualData, err := json.MarshalIndent(manualChecklist, "", "  ")
	if err != nil {
		t.Fatalf("marshal manual checklist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644); err != nil {
		t.Fatalf("write manual-checklist.json: %v", err)
	}

	// Call reviewShow
	output, err := reviewShow(rs.RunID, storeDir, false)
	if err != nil {
		t.Fatalf("reviewShow: %v", err)
	}

	// Verify output contains trust banner
	if !strings.Contains(output, "**Trust Level:** high") {
		t.Errorf("output missing trust level banner:\n%s", output)
	}

	// Verify output contains the spec title in rendered form
	if !strings.Contains(output, "Test Spec") {
		t.Errorf("output missing spec title:\n%s", output)
	}
}

// TestReviewShow_DetailsFlagIncludesArtifacts verifies that --details flag
// includes technical artifacts (validation.json, acceptance.json, review.json)
// in the output.
func TestReviewShow_DetailsFlagIncludesArtifacts(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a terminal run
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

	// Write manual-checklist.json (required by loadRunAndEnsurePacket)
	manualChecklist := map[string]interface{}{"items": []interface{}{}}
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Write validation.json
	validationArtifact := map[string]interface{}{
		"passed": true,
		"checks": 42,
	}
	validationData, _ := json.MarshalIndent(validationArtifact, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "validation.json"), validationData, 0o644)

	// Write acceptance.json
	acceptanceArtifact := map[string]interface{}{
		"passed":  10,
		"failed":  0,
		"unclear": 2,
	}
	acceptanceData, _ := json.MarshalIndent(acceptanceArtifact, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "acceptance.json"), acceptanceData, 0o644)

	// Write review.json
	reviewArtifact := map[string]interface{}{
		"findings": 3,
	}
	reviewData, _ := json.MarshalIndent(reviewArtifact, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644)

	// Call with details=false - should NOT include artifacts
	outputWithoutDetails, err := reviewShow(rs.RunID, storeDir, false)
	if err != nil {
		t.Fatalf("reviewShow without details: %v", err)
	}
	if strings.Contains(outputWithoutDetails, "Technical Artifacts") {
		t.Error("output should not contain 'Technical Artifacts' section when details=false")
	}
	if strings.Contains(outputWithoutDetails, "validation.json") {
		t.Error("output should not contain validation.json when details=false")
	}

	// Call with details=true - should include artifacts
	outputWithDetails, err := reviewShow(rs.RunID, storeDir, true)
	if err != nil {
		t.Fatalf("reviewShow with details: %v", err)
	}

	if !strings.Contains(outputWithDetails, "Technical Artifacts") {
		t.Error("output missing 'Technical Artifacts' section when details=true")
	}
	if !strings.Contains(outputWithDetails, "### validation.json") {
		t.Error("output missing validation.json artifact when details=true")
	}
	if !strings.Contains(outputWithDetails, "### acceptance.json") {
		t.Error("output missing acceptance.json artifact when details=true")
	}
	if !strings.Contains(outputWithDetails, "### review.json") {
		t.Error("output missing review.json artifact when details=true")
	}
	// Verify JSON content is included
	if !strings.Contains(outputWithDetails, `"passed": true`) {
		t.Error("validation.json content not properly included in output")
	}
}

// TestReviewShow_RefusesNonTerminalRun verifies that review show returns an error
// when attempting to review a run that is not in a terminal state (e.g., running).
func TestReviewShow_RefusesNonTerminalRun(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in non-terminal state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusRunning
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Attempt to review should fail
	output, err := reviewShow(rs.RunID, storeDir, false)
	if err == nil {
		t.Error("expected error when reviewing non-terminal run, got nil")
	}
	if !strings.Contains(err.Error(), "non-terminal") {
		t.Errorf("expected error mentioning non-terminal, got: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output on error, got: %s", output)
	}
}

// TestReviewShow_RegeneratesMissingPacket verifies that review show regenerates
// the review packet from evidence when the packet artifacts are missing.
func TestReviewShow_RegeneratesMissingPacket(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a terminal run
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

	// Create minimal spec.md for InputsFromEvidence
	runDir := store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	specPath := filepath.Join(runDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(`# Test Spec

## Vision
Test vision for the spec.

## Acceptance Criteria
- Criterion 1
`), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// Create minimal validation.json in evidence directory (needed by InputsFromEvidence)
	validationArtifact := map[string]interface{}{
		"passed": true,
		"checks": 5,
	}
	validationData, _ := json.MarshalIndent(validationArtifact, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "validation.json"), validationData, 0o644)

	// Create minimal acceptance.json
	acceptanceArtifact := map[string]interface{}{
		"passed":  5,
		"failed":  0,
		"unclear": 0,
	}
	acceptanceData, _ := json.MarshalIndent(acceptanceArtifact, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "acceptance.json"), acceptanceData, 0o644)

	// Create minimal review.json
	reviewArtifact := map[string]interface{}{
		"findings": []interface{}{},
	}
	reviewData, _ := json.MarshalIndent(reviewArtifact, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644)

	// Verify that none of the packet artifacts exist yet
	productReviewPath := filepath.Join(evidenceDir, "product-review.json")
	processReviewPath := filepath.Join(evidenceDir, "process-review.json")
	manualChecklistPath := filepath.Join(evidenceDir, "manual-checklist.json")
	if _, err := os.Stat(productReviewPath); err == nil {
		t.Fatal("product-review.json should not exist yet")
	}
	if _, err := os.Stat(processReviewPath); err == nil {
		t.Fatal("process-review.json should not exist yet")
	}
	if _, err := os.Stat(manualChecklistPath); err == nil {
		t.Fatal("manual-checklist.json should not exist yet")
	}

	// Call reviewShow - should regenerate the missing packet
	output, err := reviewShow(rs.RunID, storeDir, false)
	if err != nil {
		t.Fatalf("reviewShow: %v", err)
	}

	// Verify that all packet artifacts were created
	if _, err := os.Stat(productReviewPath); err != nil {
		t.Fatalf("product-review.json was not regenerated: %v", err)
	}
	if _, err := os.Stat(processReviewPath); err != nil {
		t.Fatalf("process-review.json was not regenerated: %v", err)
	}
	if _, err := os.Stat(manualChecklistPath); err != nil {
		t.Fatalf("manual-checklist.json was not regenerated: %v", err)
	}

	// Verify output contains content from the generated review
	if output == "" {
		t.Error("output should not be empty after regeneration")
	}

	// Verify the file has valid structure
	productData, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}
	var productReview reviewpacket.ProductReview
	if err := json.Unmarshal(productData, &productReview); err != nil {
		t.Fatalf("unmarshal product-review.json: %v", err)
	}
	if productReview.RunID != rs.RunID {
		t.Errorf("regenerated packet has wrong RunID: got %q, want %q", productReview.RunID, rs.RunID)
	}
}

// TestReviewShow_CommandLineIntegration tests the cobra command integration.
func TestReviewShow_CommandWithLatestRunID(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a terminal run
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory and minimal review packet
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	os.MkdirAll(evidenceDir, 0o755)

	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Latest Test Spec",
		TerminalState: runstore.StatusReadyForReview,
		Summary:       "Latest summary",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "medium"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := map[string]interface{}{"items": []interface{}{}}
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Test with "latest" argument
	cmd := newReviewShowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"latest", "--store-dir", storeDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "**Trust Level:** medium") {
		t.Errorf("output missing trust level from latest run:\n%s", output)
	}
}

// TestReviewShow_CommandWithExplicitRunID tests the cobra command with explicit run ID.
func TestReviewShow_CommandWithExplicitRunID(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a terminal run
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusNeedsHuman
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create evidence directory with review packet
	evidenceDir := store.RunEvidenceDir(rs.RunID)
	os.MkdirAll(evidenceDir, 0o755)

	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Explicit ID Test",
		TerminalState: runstore.StatusNeedsHuman,
		Summary:       "Needs human review",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "low"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := map[string]interface{}{"items": []interface{}{}}
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Test with explicit run ID
	cmd := newReviewShowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "**Trust Level:** low") {
		t.Errorf("output missing trust level:\n%s", output)
	}
	if !strings.Contains(output, "Explicit ID Test") {
		t.Errorf("output missing spec title:\n%s", output)
	}
}

// TestReviewShow_CommandDetailsFlagWorks tests that the --details flag is properly
// passed through the command and includes technical artifacts.
func TestReviewShow_CommandDetailsFlagWorks(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusCompleted
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)
	os.MkdirAll(evidenceDir, 0o755)

	productReview := reviewpacket.ProductReview{
		RunID:         rs.RunID,
		SpecTitle:     "Completed Spec",
		TerminalState: runstore.StatusCompleted,
		Summary:       "Work completed",
	}
	productReview.NormalizeNilFields()
	productData, _ := json.MarshalIndent(productReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "product-review.json"), productData, 0o644)

	processReview := map[string]interface{}{"trust_level": "high"}
	processData, _ := json.MarshalIndent(processReview, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "process-review.json"), processData, 0o644)

	manualChecklist := map[string]interface{}{"items": []interface{}{}}
	manualData, _ := json.MarshalIndent(manualChecklist, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "manual-checklist.json"), manualData, 0o644)

	// Write an artifact
	artifactData, _ := json.MarshalIndent(map[string]interface{}{"test": "data"}, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "validation.json"), artifactData, 0o644)

	// Test WITHOUT --details
	cmd := newReviewShowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execute: %v", err)
	}
	outputWithoutDetails := buf.String()

	// Test WITH --details
	cmd = newReviewShowCmd()
	buf.Reset()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir, "--details"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execute with --details: %v", err)
	}
	outputWithDetails := buf.String()

	// Verify --details adds Technical Artifacts section
	if strings.Contains(outputWithoutDetails, "Technical Artifacts") {
		t.Error("output without --details should not contain Technical Artifacts")
	}
	if !strings.Contains(outputWithDetails, "Technical Artifacts") {
		t.Error("output with --details should contain Technical Artifacts")
	}
}
