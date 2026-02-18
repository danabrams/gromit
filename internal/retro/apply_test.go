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
