package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestLoadRunAndEnsurePacket_RegeneratesFromEvidence verifies that
// loadRunAndEnsurePacket uses reviewpacket.InputsFromEvidence and Generator.Generate
// to regenerate missing packet artifacts.
func TestLoadRunAndEnsurePacket_RegeneratesFromEvidence(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	runID := rs.RunID
	runEvidenceDir := store.RunEvidenceDir(runID)

	// Create evidence directory with minimal required artifacts for regeneration
	if err := os.MkdirAll(runEvidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}

	// Write spec.md (required for InputsFromEvidence)
	specContent := `# Test Spec

## Acceptance Criteria
- Criterion 1
- Criterion 2
`
	runDir := store.RunDir(runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// Write validation.json (required for InputsFromEvidence)
	validation := map[string]interface{}{
		"passed": true,
		"checks": []map[string]interface{}{},
	}
	validationData, err := json.MarshalIndent(validation, "", "  ")
	if err != nil {
		t.Fatalf("marshal validation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runEvidenceDir, "validation.json"), validationData, 0o644); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write acceptance.json (required for InputsFromEvidence)
	acceptance := map[string]interface{}{
		"passed":  1,
		"failed":  0,
		"unclear": 0,
	}
	acceptanceData, err := json.MarshalIndent(acceptance, "", "  ")
	if err != nil {
		t.Fatalf("marshal acceptance: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runEvidenceDir, "acceptance.json"), acceptanceData, 0o644); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json (required for InputsFromEvidence)
	review := map[string]interface{}{
		"findings": map[string]interface{}{},
	}
	reviewData, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runEvidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Verify packet artifacts don't exist yet
	productReviewPath := filepath.Join(runEvidenceDir, "product-review.json")
	processReviewPath := filepath.Join(runEvidenceDir, "process-review.json")
	manualChecklistPath := filepath.Join(runEvidenceDir, "manual-checklist.json")

	if _, err := os.Stat(productReviewPath); err == nil {
		t.Fatalf("product-review.json should not exist before loadRunAndEnsurePacket")
	}
	if _, err := os.Stat(processReviewPath); err == nil {
		t.Fatalf("process-review.json should not exist before loadRunAndEnsurePacket")
	}
	if _, err := os.Stat(manualChecklistPath); err == nil {
		t.Fatalf("manual-checklist.json should not exist before loadRunAndEnsurePacket")
	}

	// Call loadRunAndEnsurePacket
	run, returnedStore, returnedEvidenceDir, err := loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		t.Fatalf("loadRunAndEnsurePacket: %v", err)
	}

	// Verify returned values
	if run == nil {
		t.Fatalf("returned run is nil")
	}
	if run.RunID != runID {
		t.Fatalf("returned run ID mismatch: got %q, want %q", run.RunID, runID)
	}
	if returnedStore == nil {
		t.Fatalf("returned store is nil")
	}
	if returnedEvidenceDir != runEvidenceDir {
		t.Fatalf("returned evidence dir mismatch: got %q, want %q", returnedEvidenceDir, runEvidenceDir)
	}

	// Verify packet artifacts were regenerated and written
	if _, err := os.Stat(productReviewPath); err != nil {
		t.Fatalf("product-review.json should exist after loadRunAndEnsurePacket: %v", err)
	}
	if _, err := os.Stat(processReviewPath); err != nil {
		t.Fatalf("process-review.json should exist after loadRunAndEnsurePacket: %v", err)
	}
	if _, err := os.Stat(manualChecklistPath); err != nil {
		t.Fatalf("manual-checklist.json should exist after loadRunAndEnsurePacket: %v", err)
	}

	// Verify the artifacts are valid JSON and match expected types
	productData, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}
	var productReview reviewpacket.ProductReview
	if err := json.Unmarshal(productData, &productReview); err != nil {
		t.Fatalf("parse product-review.json: %v", err)
	}
	if productReview.RunID != runID {
		t.Fatalf("product review run ID mismatch: got %q, want %q", productReview.RunID, runID)
	}

	processData, err := os.ReadFile(processReviewPath)
	if err != nil {
		t.Fatalf("read process-review.json: %v", err)
	}
	var processReview reviewpacket.ProcessReview
	if err := json.Unmarshal(processData, &processReview); err != nil {
		t.Fatalf("parse process-review.json: %v", err)
	}

	manualData, err := os.ReadFile(manualChecklistPath)
	if err != nil {
		t.Fatalf("read manual-checklist.json: %v", err)
	}
	var manualChecklist reviewpacket.ManualChecklist
	if err := json.Unmarshal(manualData, &manualChecklist); err != nil {
		t.Fatalf("parse manual-checklist.json: %v", err)
	}
}

// TestLoadRunAndEnsurePacket_SkipsRegenerationWhenArtifactsExist verifies that
// loadRunAndEnsurePacket returns early when all packet artifacts already exist.
func TestLoadRunAndEnsurePacket_SkipsRegenerationWhenArtifactsExist(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "my-project")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	runID := rs.RunID
	runEvidenceDir := store.RunEvidenceDir(runID)

	// Create evidence directory with all packet artifacts
	if err := os.MkdirAll(runEvidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}

	// Write complete packet artifacts
	productReview := reviewpacket.ProductReview{
		RunID:         runID,
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
	if err := os.WriteFile(filepath.Join(runEvidenceDir, "product-review.json"), productData, 0o644); err != nil {
		t.Fatalf("write product-review.json: %v", err)
	}

	processReview := reviewpacket.ProcessReview{}
	processReview.NormalizeNilFields()
	processData, err := json.MarshalIndent(processReview, "", "  ")
	if err != nil {
		t.Fatalf("marshal process review: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runEvidenceDir, "process-review.json"), processData, 0o644); err != nil {
		t.Fatalf("write process-review.json: %v", err)
	}

	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{},
	}
	manualChecklist.NormalizeNilFields()
	manualData, err := json.MarshalIndent(manualChecklist, "", "  ")
	if err != nil {
		t.Fatalf("marshal manual checklist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runEvidenceDir, "manual-checklist.json"), manualData, 0o644); err != nil {
		t.Fatalf("write manual-checklist.json: %v", err)
	}

	// Call loadRunAndEnsurePacket
	run, returnedStore, returnedEvidenceDir, err := loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		t.Fatalf("loadRunAndEnsurePacket: %v", err)
	}

	// Verify returned values
	if run == nil {
		t.Fatalf("returned run is nil")
	}
	if run.RunID != runID {
		t.Fatalf("returned run ID mismatch: got %q, want %q", run.RunID, runID)
	}
	if returnedStore == nil {
		t.Fatalf("returned store is nil")
	}
	if returnedEvidenceDir != runEvidenceDir {
		t.Fatalf("returned evidence dir mismatch: got %q, want %q", returnedEvidenceDir, runEvidenceDir)
	}
}
