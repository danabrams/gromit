package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func mustSaveSpec(t *testing.T, store *runstore.Store, rs *runstore.RunState) {
	t.Helper()
	if err := store.Save(rs); err != nil {
		t.Fatalf("save %s: %v", rs.RunID, err)
	}
}

func TestScenario_SpecList_DoneSpecsAtBottomWithDate(t *testing.T) {
	// Seed: create specs directory with three spec files
	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	for _, name := range []string{"0003a", "0003h", "0003i"} {
		path := filepath.Join(specsDir, name+".md")
		content := "# Spec " + name + "\n"
		if name == "0003a" {
			content = "DONE 2026-03-19\n" + content
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Seed: create store with runs
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// 0003a — completed (done) on 2026-03-19
	mustSaveSpec(t, store, &runstore.RunState{
		RunID:     "run-done-0003a",
		SpecID:    "0003a",
		ProjectID: "testproj",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	})

	// 0003h — no runs, so status = ready
	// (no run seeded)

	// 0003i — ready_for_review
	mustSaveSpec(t, store, &runstore.RunState{
		RunID:     "run-review-0003i",
		SpecID:    "0003i",
		ProjectID: "testproj",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Date(2026, 3, 19, 8, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	})

	// Invoke: call spec list via cobra
	cmd := newSpecListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--project", "testproj",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}
	output := buf.String()

	// Assert: 0003h and 0003i appear before 0003a
	idx0003a := strings.Index(output, "0003a")
	idx0003h := strings.Index(output, "0003h")
	idx0003i := strings.Index(output, "0003i")

	if idx0003a < 0 {
		t.Fatal("expected 0003a in output")
	}
	if idx0003h < 0 {
		t.Fatal("expected 0003h in output")
	}
	if idx0003i < 0 {
		t.Fatal("expected 0003i in output")
	}

	if idx0003h > idx0003a {
		t.Errorf("expected 0003h (at %d) before 0003a (at %d) in output:\n%s", idx0003h, idx0003a, output)
	}
	if idx0003i > idx0003a {
		t.Errorf("expected 0003i (at %d) before 0003a (at %d) in output:\n%s", idx0003i, idx0003a, output)
	}

	// Assert: 0003a shows done status with date
	if !strings.Contains(output, "done (2026-03-19)") {
		t.Errorf("expected 0003a to show 'done (2026-03-19)' status in output:\n%s", output)
	}

	// Assert: non-done specs show their expected statuses
	// Find the line for 0003h and check it shows "ready"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "0003h") {
			if !strings.Contains(line, "ready") {
				t.Errorf("expected 0003h line to contain 'ready', got: %s", line)
			}
		}
		if strings.Contains(line, "0003i") {
			if !strings.Contains(line, "ready_for_review") {
				t.Errorf("expected 0003i line to contain 'ready_for_review', got: %s", line)
			}
		}
	}
}
