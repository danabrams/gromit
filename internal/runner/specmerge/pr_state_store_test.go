package specmerge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/review"
)

func TestPRStateStoreFilePersistsAndRehydrates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	store, err := NewPRStateStoreFile(gromitDir)
	if err != nil {
		t.Fatalf("NewPRStateStoreFile: %v", err)
	}

	state := &PRState{
		SpecName:         "auth",
		PRRef:            PRRef{Owner: "acme", Repo: "specs", Number: 42},
		Outcome:          PROutcomeMerged,
		AwaitingApproval: true,
		FixCycleCount:    2,
		StageResults: []StageResult{
			{
				StageName: "spec_conformance",
				Tier:      "tier-high",
				Passed:    false,
				ReviewResult: &review.ReviewResult{
					Passed:  false,
					Summary: "disagreements",
				},
			},
		},
		LastUpdated: time.Now().UTC(),
	}

	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	stateFilePath := filepath.Join(gromitDir, "spec-pr-state.json")
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	raw["future_flag"] = json.RawMessage("\"enabled\"")

	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(stateFilePath, updated, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reloaded, err := NewPRStateStoreFile(gromitDir)
	if err != nil {
		t.Fatalf("NewPRStateStoreFile (reload): %v", err)
	}

	states, err := reloaded.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("len(states) = %d, want 1", len(states))
	}

	got := states[0]
	if got.PRRef.Number != 42 {
		t.Fatalf("PR number = %d, want 42", got.PRRef.Number)
	}
	if got.Outcome != PROutcomeMerged {
		t.Fatalf("Outcome = %v, want %v", got.Outcome, PROutcomeMerged)
	}
	if !got.AwaitingApproval {
		t.Fatal("expected awaiting approval")
	}
	if got.FixCycleCount != 2 {
		t.Fatalf("FixCycleCount = %d, want 2", got.FixCycleCount)
	}
	if len(got.StageResults) != 1 {
		t.Fatalf("StageResults len = %d, want 1", len(got.StageResults))
	}
	if got.StageResults[0].ReviewResult == nil {
		t.Fatal("expected review result")
	}
	if got.StageResults[0].ReviewResult.Summary != "disagreements" {
		t.Fatalf("summary = %q, want disagreements", got.StageResults[0].ReviewResult.Summary)
	}
}
