package retro

import (
	"os"
	"path/filepath"
	"strings"
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
	l1Hash := l1.Hash
	l2, err := lf.Add("bead-2", "Learning two", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning two: %v", err)
	}
	l2Hash := l2.Hash

	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{l1Hash, l2Hash},
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

	if got := reloaded.GetByHash(l1Hash); got != nil {
		t.Fatalf("expected first learning removed")
	}
	if got := reloaded.GetByHash(l2Hash); got != nil {
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

func TestApplyProposals_ArchivesLearning(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	if err := os.WriteFile(rulesPath, []byte("# Rules\n\n## Safety\n\n- Rule A\n"), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file: %v", err)
	}
	learning, err := lf.Add("bead-1", "Learning one", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning: %v", err)
	}

	proposals := &Proposals{
		Archives: []ArchiveProposal{
			{
				LearningHash: learning.Hash,
				Rationale:    "no longer relevant",
			},
		},
	}

	if err := ApplyProposals(proposals, lf, rulesPath); err != nil {
		t.Fatalf("apply proposals: %v", err)
	}

	archivedContent, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("read learnings file: %v", err)
	}
	if !strings.Contains(string(archivedContent), "Learning one") {
		t.Fatalf("expected archived learning content to remain")
	}
	if !strings.Contains(string(archivedContent), "Archived from") {
		t.Fatalf("expected archive note in learnings file")
	}
	if !strings.Contains(string(archivedContent), "no longer relevant") {
		t.Fatalf("expected archive rationale in learnings file")
	}
}

func TestApplyProposals_PromotesLearningToRule(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	rulesContent := "# Rules\n\n## Code Style\n\n- Rule A\n\n## Safety\n\n- Rule B\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file: %v", err)
	}
	learning, err := lf.Add("bead-1", "Learning one", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning: %v", err)
	}

	proposals := &Proposals{
		Promotions: []PromotionProposal{
			{
				LearningHash: learning.Hash,
				ProposedRule: "- Use go fmt",
				Section:      "Code Style",
				Rationale:    "consistency",
			},
		},
	}

	if err := ApplyProposals(proposals, lf, rulesPath); err != nil {
		t.Fatalf("apply proposals: %v", err)
	}

	updatedRules, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rulesText := string(updatedRules)
	if !strings.Contains(rulesText, "- Use go fmt") {
		t.Fatalf("expected promoted rule appended to rules")
	}
	codeStyleIdx := strings.Index(rulesText, "## Code Style")
	promoIdx := strings.Index(rulesText, "- Use go fmt")
	safetyIdx := strings.Index(rulesText, "## Safety")
	if !(codeStyleIdx < promoIdx && promoIdx < safetyIdx) {
		t.Fatalf("expected promoted rule in Code Style section")
	}

	learningsContent, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("read learnings: %v", err)
	}
	t.Logf("learnings:\n%s", string(learningsContent))
	t.Logf("learnings:\n%s", string(learningsContent))
	if !strings.Contains(string(learningsContent), "Archived from") {
		t.Fatalf("expected archived learning after promotion")
	}
	if !strings.Contains(string(learningsContent), "consistency") {
		t.Fatalf("expected archive rationale in learnings file")
	}
}

func TestApplyProposals_UpdatesRuleText(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	rulesContent := "# Rules\n\n## Process\n\n- Old rule\n- Old rule\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file: %v", err)
	}

	proposals := &Proposals{
		RuleChanges: []RuleChangeProposal{
			{
				CurrentRule:  "- Old rule",
				ProposedRule: "- New rule",
				Rationale:    "clarify",
			},
		},
	}

	if err := ApplyProposals(proposals, lf, rulesPath); err != nil {
		t.Fatalf("apply proposals: %v", err)
	}

	updatedRules, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rulesText := string(updatedRules)
	if strings.Contains(rulesText, "- Old rule") {
		t.Fatalf("expected old rule removed")
	}
	if count := strings.Count(rulesText, "- New rule"); count != 2 {
		t.Fatalf("expected new rule twice, got %d", count)
	}
}

func TestApplyProposals_MixedProposals(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	rulesContent := "# Rules\n\n## Code Style\n\n- Old Rule\n\n## Safety\n\n- Rule B\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
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
	l1Hash := l1.Hash
	l2, err := lf.Add("bead-2", "Learning two", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning two: %v", err)
	}
	l2Hash := l2.Hash
	l4, err := lf.Add("bead-4", "Learning four unique", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning four: %v", err)
	}
	l4Hash := l4.Hash
	l3, err := lf.Add("bead-3", "Learning three unique", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add learning three: %v", err)
	}
	l3Hash := l3.Hash

	proposals := &Proposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{l1Hash, l2Hash},
				ConsolidatedText: "Merged learning",
				Rationale:        "duplicate",
			},
		},
		Archives: []ArchiveProposal{
			{
				LearningHash: l3Hash,
				Rationale:    "obsolete",
			},
		},
		Promotions: []PromotionProposal{
			{
				LearningHash: l4Hash,
				ProposedRule: "- New Rule",
				Section:      "Code Style",
				Rationale:    "promote",
			},
		},
		RuleChanges: []RuleChangeProposal{
			{
				CurrentRule:  "- Old Rule",
				ProposedRule: "- Updated Rule",
				Rationale:    "clarify",
			},
		},
	}

	if err := ApplyProposals(proposals, lf, rulesPath); err != nil {
		t.Fatalf("apply proposals: %v", err)
	}

	updatedRules, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rulesText := string(updatedRules)
	if strings.Contains(rulesText, "- Old Rule") {
		t.Fatalf("expected old rule removed")
	}
	if !strings.Contains(rulesText, "- Updated Rule") {
		t.Fatalf("expected updated rule")
	}
	if !strings.Contains(rulesText, "- New Rule") {
		t.Fatalf("expected promoted rule")
	}

	reloaded, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file reload: %v", err)
	}
	if err := reloaded.Load(); err != nil {
		t.Fatalf("load learnings: %v", err)
	}

	foundMerged := false
	for _, learning := range reloaded.GetConfirmed() {
		if learning.Content == "Merged learning" {
			foundMerged = true
		}
	}
	if !foundMerged {
		t.Fatalf("expected consolidated learning")
	}

	learningsContent, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("read learnings: %v", err)
	}
	if !strings.Contains(string(learningsContent), "Learning three unique") {
		t.Fatalf("expected archived learning present")
	}
	if !strings.Contains(string(learningsContent), "obsolete") {
		t.Fatalf("expected archive rationale")
	}
	if !strings.Contains(string(learningsContent), "promote") {
		t.Fatalf("expected promotion rationale archived")
	}
}

func TestApplyProposals_ReturnsErrorOnMissingRuleChange(t *testing.T) {
	tmpDir := t.TempDir()

	rulesPath := filepath.Join(tmpDir, "RULES.md")
	rulesContent := "# Rules\n\n## Process\n\n- Existing rule\n"
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("new learnings file: %v", err)
	}

	proposals := &Proposals{
		RuleChanges: []RuleChangeProposal{
			{
				CurrentRule:  "- Missing rule",
				ProposedRule: "- New rule",
				Rationale:    "clarify",
			},
		},
	}

	if err := ApplyProposals(proposals, lf, rulesPath); err == nil {
		t.Fatalf("expected error for missing rule change")
	}
}
