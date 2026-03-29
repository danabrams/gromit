package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_LocalSupersededEntryMasksGlobalDoctrineRule(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.ReplanContext = &runstore.ReplanContext{Failures: []string{}}
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("spec content"), 0o644)

	// Create global doctrine with an active rule
	storeRootDir := t.TempDir()
	globalDoctrineDir := filepath.Join(storeRootDir, "global", "doctrine")
	os.MkdirAll(globalDoctrineDir, 0o755)
	globalStore := &doctrine.FSStore{Dir: globalDoctrineDir}
	globalDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:        "promoted-abcd1234",
				Summary:   "Global rule that should be masked",
				Scope:     "*",
				Status:    "active",
				CreatedAt: time.Now(),
			},
			{
				ID:        "promoted-other5678",
				Summary:   "Another global rule that should survive",
				Scope:     "api",
				Status:    "active",
				CreatedAt: time.Now(),
			},
		},
	}
	if err := globalStore.Save(globalDoctrine); err != nil {
		t.Fatalf("save global doctrine: %v", err)
	}

	// Create local doctrine with a superseded entry for the same ID
	cellPath := t.TempDir()
	localDoctrineDir := filepath.Join(cellPath, "doctrine")
	os.MkdirAll(localDoctrineDir, 0o755)
	localStore := &doctrine.FSStore{Dir: localDoctrineDir}
	localDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:           "promoted-abcd1234",
				Summary:      "Global rule that should be masked",
				Scope:        "*",
				Status:       "superseded",
				SupersededBy: "local-decision-xyz",
				CreatedAt:    time.Now(),
			},
		},
	}
	if err := localStore.Save(localDoctrine); err != nil {
		t.Fatalf("save local doctrine: %v", err)
	}

	// Also create an empty local playbook so loadPlaybookAndDoctrine doesn't short-circuit
	localPlaybookDir := filepath.Join(cellPath, "playbook")
	os.MkdirAll(localPlaybookDir, 0o755)

	// === Invoke ===
	fp := &fakePlanner{plans: []planner.Plan{validPlan()}}
	stage := NewPlanStage(fp, store, nil)
	stage.SetCellPathResolver(&fakeCellPathResolver{path: cellPath})
	stage.SetStoreRootDir(storeRootDir)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// === Assert ===
	if len(fp.reqs) != 1 {
		t.Fatalf("expected 1 plan request, got %d", len(fp.reqs))
	}
	req := fp.reqs[0]

	// The masked global rule must NOT appear in the doctrine text
	if strings.Contains(req.DoctrineRules, "Global rule that should be masked") {
		t.Fatalf("superseded global rule should not appear in DoctrineRules, got: %q", req.DoctrineRules)
	}

	// The other global rule MUST still appear
	if !strings.Contains(req.DoctrineRules, "Another global rule that should survive") {
		t.Fatalf("non-superseded global rule should appear in DoctrineRules, got: %q", req.DoctrineRules)
	}
}
