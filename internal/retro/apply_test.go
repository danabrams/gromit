package retro

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
)

func TestApplyProposals_EmptyNoChanges(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	rulesContent := "# Rules\n\n## Code Style\n\n- Rule A\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file: %v", err)
	}
	if _, err := lf.Add("bead-1", "Learning one", learnings.CategoryPatterns); err != nil {
		t.Fatalf("add learning: %v", err)
	}

	beforeRules, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	beforeLearnings, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("read learnings: %v", err)
	}

	proposals := &Proposals{}

	if err := ApplyProposals(proposals, lf, rulesPath); err != nil {
		t.Fatalf("apply proposals: %v", err)
	}

	afterRules, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read rules after: %v", err)
	}
	if string(afterRules) != string(beforeRules) {
		t.Fatalf("rules changed unexpectedly")
	}

	afterLearnings, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("read learnings after: %v", err)
	}
	if string(afterLearnings) != string(beforeLearnings) {
		t.Fatalf("learnings changed unexpectedly")
	}
}

func TestApplyProposals_ConsolidationReplacesLearnings(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	if err := os.WriteFile(rulesPath, []byte("# Rules\n\n## Code Style\n\n- Rule A\n"), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file: %v", err)
	}
	l1, err := lf.Add("bead-1", "Learning one", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning one: %v", err)
	}
	l2, err := lf.Add("bead-2", "Learning two", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning two: %v", err)
	}

	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{l1.Hash, l2.Hash},
				ConsolidatedText: "Merged learning",
				Rationale:        "duplicate",
			},
		},
	}

	if err := ApplyProposals(proposals, lf, rulesPath); err != nil {
		t.Fatalf("apply proposals: %v", err)
	}

	reloaded, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file reload: %v", err)
	}
	if err := reloaded.Load(); err != nil {
		t.Fatalf("load learnings: %v", err)
	}

	if got := reloaded.GetByHash(l1.Hash); got != nil {
		t.Fatalf("expected first learning removed")
	}
	if got := reloaded.GetByHash(l2.Hash); got != nil {
		t.Fatalf("expected second learning removed")
	}

	found := false
	for _, learning := range reloaded.GetConfirmed() {
		if learning.Content == "Merged learning" {
			found = true
			if learning.Category != learnings.CategoryPatterns {
				t.Fatalf("expected category %q, got %q", learnings.CategoryPatterns, learning.Category)
			}
		}
	}
	if !found {
		t.Fatalf("expected consolidated learning in confirmed")
	}
}
