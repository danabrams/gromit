package stages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// fakeDiffProvider is declared in review_test.go (same package).

func TestEffectiveStatus_AlreadyTerminal(t *testing.T) {
	for _, status := range []string{
		runstore.StatusReadyForReview,
		runstore.StatusNeedsHuman,
		runstore.StatusBlocked,
	} {
		rs := &runstore.RunState{Status: status}
		if got := effectiveStatus(rs); got != status {
			t.Errorf("status %q: want %q, got %q", status, status, got)
		}
	}
}

func TestEffectiveStatus_RunningAllPass_ReturnsReadyForReview(t *testing.T) {
	rs := &runstore.RunState{
		Status:                runstore.StatusRunning,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{{Status: "done"}, {Status: "done"}},
	}
	if got := effectiveStatus(rs); got != runstore.StatusReadyForReview {
		t.Errorf("want ready_for_review, got %q", got)
	}
}

func TestEffectiveStatus_RunningValidationFailed_ReturnsNeedsHuman(t *testing.T) {
	rs := &runstore.RunState{
		Status:                runstore.StatusRunning,
		FinalValidationPassed: false,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{{Status: "done"}},
	}
	if got := effectiveStatus(rs); got != runstore.StatusNeedsHuman {
		t.Errorf("want needs_human, got %q", got)
	}
}

func TestEffectiveStatus_RunningTaskNotDone_ReturnsNeedsHuman(t *testing.T) {
	rs := &runstore.RunState{
		Status:                runstore.StatusRunning,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{{Status: "done"}, {Status: "failed"}},
	}
	if got := effectiveStatus(rs); got != runstore.StatusNeedsHuman {
		t.Errorf("want needs_human, got %q", got)
	}
}

func TestEvidenceStage_ReadsReviewJSONFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)
	rs := runstore.NewRunState("test-spec", "test-project")
	store.Save(rs)

	// Write review.json to evidence directory (as ReviewStage would)
	evidenceDir := filepath.Join(store.RunDir(rs.RunID), "evidence")
	os.MkdirAll(evidenceDir, 0o755)
	reviewData := map[string][]map[string]interface{}{
		"spec_alignment": {
			{"facet": "spec_alignment", "severity": "error", "file": "handler.go", "line": 42, "description": "missing validation"},
		},
	}
	data, _ := json.MarshalIndent(reviewData, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "review.json"), data, 0o644)

	stage := NewEvidenceStage(store, EvidenceStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "test diff"},
		StartTime:    time.Now(),
	})

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// review.md should incorporate content from review.json
	mdData, err := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
	if err != nil {
		t.Fatalf("read review.md: %v", err)
	}
	content := string(mdData)
	if !strings.Contains(content, "spec_alignment") {
		t.Error("review.md should contain review findings from review.json")
	}
	if !strings.Contains(content, "1 error") {
		t.Error("review.md should contain aggregated severity counts from review.json")
	}
}

func TestEvidenceStage_ReadsAcceptanceJSONFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)
	rs := runstore.NewRunState("test-spec", "test-project")
	store.Save(rs)

	// Write acceptance.json to evidence directory (as AcceptStage would)
	evidenceDir := filepath.Join(store.RunDir(rs.RunID), "evidence")
	os.MkdirAll(evidenceDir, 0o755)
	acceptData := map[string]interface{}{
		"results": []map[string]string{
			{"criterion": "multi-currency", "status": "fail", "rationale": "only USD supported"},
		},
		"all_pass":            false,
		"has_fail_or_unclear": true,
	}
	data, _ := json.MarshalIndent(acceptData, "", "  ")
	os.WriteFile(filepath.Join(evidenceDir, "acceptance.json"), data, 0o644)

	stage := NewEvidenceStage(store, EvidenceStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "test diff"},
		StartTime:    time.Now(),
	})

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mdData, err := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
	if err != nil {
		t.Fatalf("read review.md: %v", err)
	}
	content := string(mdData)
	if !strings.Contains(content, "multi-currency") {
		t.Error("review.md should contain acceptance results from acceptance.json")
	}
}

func TestEvidenceStage_MalformedJSON_NotEvaluatedFallback(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)
	rs := runstore.NewRunState("test-spec", "test-project")
	store.Save(rs)

	evidenceDir := filepath.Join(store.RunDir(rs.RunID), "evidence")
	os.MkdirAll(evidenceDir, 0o755)
	os.WriteFile(filepath.Join(evidenceDir, "review.json"), []byte("{invalid json"), 0o644)
	os.WriteFile(filepath.Join(evidenceDir, "acceptance.json"), []byte("{invalid json"), 0o644)

	stage := NewEvidenceStage(store, EvidenceStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "test diff"},
		StartTime:    time.Now(),
	})

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
	content := string(data)
	if !strings.Contains(content, "Not evaluated") {
		t.Error("review.md should contain 'Not evaluated' when JSON files are malformed")
	}
}

func TestEvidenceStage_MissingFiles_NotEvaluatedSections(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)
	rs := runstore.NewRunState("test-spec", "test-project")
	store.Save(rs)

	// No review.json or acceptance.json on disk

	stage := NewEvidenceStage(store, EvidenceStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "test diff"},
		StartTime:    time.Now(),
	})

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	evidenceDir := filepath.Join(store.RunDir(rs.RunID), "evidence")
	data, _ := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
	content := string(data)
	if !strings.Contains(content, "Not evaluated") {
		t.Error("review.md should contain 'Not evaluated' when review.json and acceptance.json are missing")
	}
}
