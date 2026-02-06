package retro

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/learnings"
	"github.com/danabrams/ralph-runner/internal/rules"
)

func TestNewRetroNilConfig(t *testing.T) {
	r := NewRetro(nil, ".ralph")
	if r != nil {
		t.Error("expected nil Retro for nil config")
	}
}

func TestRunNilReceiver(t *testing.T) {
	var r *Retro
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil retro")
	}
}

func TestRunNilClaudeClient(t *testing.T) {
	r := &Retro{
		claude: nil,
	}
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil claude client")
	}
}

func TestRunNilLearningsFile(t *testing.T) {
	r := &Retro{
		claude:        claude.NewClient("claude", nil, 60),
		learningsFile: nil,
	}
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil learnings file")
	}
}

func TestApplyAcceptedNil(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Retro{
		learningsFile: learnings.NewFile(tmpDir),
		rulesPath:     filepath.Join(tmpDir, "RULES.md"),
	}

	err := r.applyAccepted(nil)
	if err == nil {
		t.Error("expected error for nil accepted proposals")
	}
}

func TestApplyConsolidation(t *testing.T) {
	tmpDir := t.TempDir()
	learningsFile := learnings.NewFile(tmpDir)

	// Add some test learnings
	learningsFile.Add("bead-1", "Learning one content", learnings.CategoryPatterns)
	learningsFile.Add("bead-2", "Learning two content", learnings.CategoryPatterns)

	r := &Retro{
		learningsFile: learningsFile,
		rulesPath:     filepath.Join(tmpDir, "RULES.md"),
	}

	// Get hashes of the learnings we just added
	confirmed := learningsFile.GetConfirmed()
	provisional := learningsFile.GetProvisional()
	allLearnings := append(confirmed, provisional...)

	if len(allLearnings) < 2 {
		t.Fatalf("expected at least 2 learnings, got %d", len(allLearnings))
	}

	hashes := []string{allLearnings[0].Hash, allLearnings[1].Hash}

	// Apply consolidation
	c := ConsolidationProposal{
		LearningHashes:   hashes,
		ConsolidatedText: "Consolidated learning content",
		Rationale:        "These learnings are similar",
	}

	if err := r.applyConsolidation(c); err != nil {
		t.Errorf("applyConsolidation failed: %v", err)
	}

	// Verify consolidation happened
	if err := learningsFile.Load(); err != nil {
		t.Fatalf("failed to reload learnings: %v", err)
	}

	// Check that consolidated learning exists
	confirmed = learningsFile.GetConfirmed()
	provisional = learningsFile.GetProvisional()
	allLearnings = append(confirmed, provisional...)

	found := false
	for _, l := range allLearnings {
		if strings.Contains(l.Content, "Consolidated learning content") {
			found = true
			break
		}
	}

	if !found {
		t.Error("consolidated learning not found")
	}
}

func TestApplyArchive(t *testing.T) {
	tmpDir := t.TempDir()
	learningsFile := learnings.NewFile(tmpDir)

	// Add a test learning
	learningsFile.Add("bead-1", "Learning to archive", learnings.CategoryPatterns)

	r := &Retro{
		learningsFile: learningsFile,
		rulesPath:     filepath.Join(tmpDir, "RULES.md"),
	}

	// Get the hash of the learning
	confirmed := learningsFile.GetConfirmed()
	provisional := learningsFile.GetProvisional()
	allLearnings := append(confirmed, provisional...)

	if len(allLearnings) == 0 {
		t.Fatal("expected at least 1 learning")
	}

	hash := allLearnings[0].Hash

	// Apply archive
	a := ArchiveProposal{
		LearningHash: hash,
		Rationale:    "No longer relevant",
	}

	if err := r.applyArchive(a); err != nil {
		t.Errorf("applyArchive failed: %v", err)
	}

	// Verify learning was archived
	if err := learningsFile.Load(); err != nil {
		t.Fatalf("failed to reload learnings: %v", err)
	}

	// Check that learning is no longer in confirmed/provisional
	confirmed = learningsFile.GetConfirmed()
	provisional = learningsFile.GetProvisional()
	for _, l := range append(confirmed, provisional...) {
		if l.Hash == hash {
			t.Error("learning should have been archived but is still active")
		}
	}
}

func TestApplyPromotion(t *testing.T) {
	tmpDir := t.TempDir()
	learningsFile := learnings.NewFile(tmpDir)

	// Add a test learning
	learningsFile.Add("bead-1", "Learning to promote", learnings.CategoryPatterns)

	// Create initial rules file
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	initialRules := &rules.Rules{
		Sections: []rules.Section{
			{
				Name:  "Code Style",
				Rules: []string{"Existing rule"},
			},
		},
	}
	if err := initialRules.Save(rulesPath); err != nil {
		t.Fatalf("failed to create initial rules: %v", err)
	}

	r := &Retro{
		learningsFile: learningsFile,
		rulesPath:     rulesPath,
	}

	// Get the hash of the learning
	confirmed := learningsFile.GetConfirmed()
	provisional := learningsFile.GetProvisional()
	allLearnings := append(confirmed, provisional...)

	if len(allLearnings) == 0 {
		t.Fatal("expected at least 1 learning")
	}

	hash := allLearnings[0].Hash

	// Apply promotion
	p := PromotionProposal{
		LearningHash: hash,
		ProposedRule: "New rule from learning",
		Section:      "Code Style",
		Rationale:    "This should be a rule",
	}

	if err := r.applyPromotion(p); err != nil {
		t.Errorf("applyPromotion failed: %v", err)
	}

	// Verify rule was added
	updatedRules, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("failed to load updated rules: %v", err)
	}

	found := false
	for _, section := range updatedRules.Sections {
		if section.Name == "Code Style" {
			for _, rule := range section.Rules {
				if rule == "New rule from learning" {
					found = true
					break
				}
			}
		}
	}

	if !found {
		t.Error("promoted rule not found in rules file")
	}
}

func TestApplyRuleChange(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial rules file
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	initialRules := &rules.Rules{
		Sections: []rules.Section{
			{
				Name:  "Code Style",
				Rules: []string{"Old rule text"},
			},
		},
	}
	if err := initialRules.Save(rulesPath); err != nil {
		t.Fatalf("failed to create initial rules: %v", err)
	}

	r := &Retro{
		learningsFile: learnings.NewFile(tmpDir),
		rulesPath:     rulesPath,
	}

	// Apply rule change
	rc := RuleChangeProposal{
		CurrentRule:  "Old rule text",
		ProposedRule: "New rule text",
		Rationale:    "Better wording",
	}

	if err := r.applyRuleChange(rc); err != nil {
		t.Errorf("applyRuleChange failed: %v", err)
	}

	// Verify rule was changed
	updatedRules, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("failed to load updated rules: %v", err)
	}

	found := false
	for _, section := range updatedRules.Sections {
		for _, rule := range section.Rules {
			if rule == "New rule text" {
				found = true
			}
			if rule == "Old rule text" {
				t.Error("old rule text should have been replaced")
			}
		}
	}

	if !found {
		t.Error("new rule text not found in rules file")
	}
}

func TestApplyAcceptedWithAllOperations(t *testing.T) {
	tmpDir := t.TempDir()
	learningsFile := learnings.NewFile(tmpDir)

	// Add test learnings
	learningsFile.Add("bead-1", "Learning one", learnings.CategoryPatterns)
	learningsFile.Add("bead-2", "Learning two", learnings.CategoryPatterns)
	learningsFile.Add("bead-3", "Learning three", learnings.CategoryGotchas)

	// Create initial rules file
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	initialRules := &rules.Rules{
		Sections: []rules.Section{
			{
				Name:  "Code Style",
				Rules: []string{"Old rule"},
			},
		},
	}
	if err := initialRules.Save(rulesPath); err != nil {
		t.Fatalf("failed to create initial rules: %v", err)
	}

	r := &Retro{
		learningsFile: learningsFile,
		rulesPath:     rulesPath,
	}

	// Get learning hashes
	confirmed := learningsFile.GetConfirmed()
	provisional := learningsFile.GetProvisional()
	allLearnings := append(confirmed, provisional...)

	if len(allLearnings) < 3 {
		t.Fatalf("expected at least 3 learnings, got %d", len(allLearnings))
	}

	// Create accepted proposals with all operation types
	accepted := &AcceptedProposals{
		Consolidations: []ConsolidationProposal{
			{
				LearningHashes:   []string{allLearnings[0].Hash, allLearnings[1].Hash},
				ConsolidatedText: "Consolidated learning",
				Rationale:        "Similar patterns",
			},
		},
		Archives: []ArchiveProposal{
			{
				LearningHash: allLearnings[2].Hash,
				Rationale:    "No longer relevant",
			},
		},
		Promotions: []PromotionProposal{
			{
				LearningHash: allLearnings[0].Hash, // This will fail since it's being consolidated
				ProposedRule: "Promoted rule",
				Section:      "Code Style",
				Rationale:    "Should be a rule",
			},
		},
		RuleChanges: []RuleChangeProposal{
			{
				CurrentRule:  "Old rule",
				ProposedRule: "Updated rule",
				Rationale:    "Better wording",
			},
		},
	}

	// Apply all operations
	err := r.applyAccepted(accepted)

	// We expect some operations to succeed and some to fail (e.g., promotion of already consolidated learning)
	// The function should continue and report errors
	if err != nil && !strings.Contains(err.Error(), "some operations failed") {
		t.Errorf("unexpected error type: %v", err)
	}

	// Verify at least some operations succeeded by checking the rules file
	updatedRules, err := rules.Load(rulesPath)
	if err != nil {
		t.Fatalf("failed to load updated rules: %v", err)
	}

	foundUpdatedRule := false
	for _, section := range updatedRules.Sections {
		for _, rule := range section.Rules {
			if rule == "Updated rule" {
				foundUpdatedRule = true
			}
		}
	}

	if !foundUpdatedRule {
		t.Error("rule change did not succeed")
	}
}

func TestApplyConsolidationEmptyHashes(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Retro{
		learningsFile: learnings.NewFile(tmpDir),
		rulesPath:     filepath.Join(tmpDir, "RULES.md"),
	}

	c := ConsolidationProposal{
		LearningHashes:   []string{},
		ConsolidatedText: "Text",
		Rationale:        "Rationale",
	}

	err := r.applyConsolidation(c)
	if err == nil {
		t.Error("expected error for empty learning hashes")
	}
}

func TestApplyArchiveEmptyHash(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Retro{
		learningsFile: learnings.NewFile(tmpDir),
		rulesPath:     filepath.Join(tmpDir, "RULES.md"),
	}

	a := ArchiveProposal{
		LearningHash: "",
		Rationale:    "Rationale",
	}

	err := r.applyArchive(a)
	if err == nil {
		t.Error("expected error for empty learning hash")
	}
}

func TestApplyPromotionMissingFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial rules file
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	initialRules := &rules.Rules{
		Sections: []rules.Section{
			{Name: "Code Style", Rules: []string{}},
		},
	}
	if err := initialRules.Save(rulesPath); err != nil {
		t.Fatalf("failed to create initial rules: %v", err)
	}

	r := &Retro{
		learningsFile: learnings.NewFile(tmpDir),
		rulesPath:     rulesPath,
	}

	tests := []struct {
		name     string
		proposal PromotionProposal
	}{
		{
			name: "missing hash",
			proposal: PromotionProposal{
				LearningHash: "",
				ProposedRule: "Rule",
				Section:      "Code Style",
			},
		},
		{
			name: "missing rule",
			proposal: PromotionProposal{
				LearningHash: "abc123",
				ProposedRule: "",
				Section:      "Code Style",
			},
		},
		{
			name: "missing section",
			proposal: PromotionProposal{
				LearningHash: "abc123",
				ProposedRule: "Rule",
				Section:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.applyPromotion(tt.proposal)
			if err == nil {
				t.Error("expected error for missing field")
			}
		})
	}
}

func TestApplyRuleChangeMissingFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial rules file
	rulesPath := filepath.Join(tmpDir, "RULES.md")
	if err := os.WriteFile(rulesPath, []byte("# Rules\n"), 0644); err != nil {
		t.Fatalf("failed to create rules file: %v", err)
	}

	r := &Retro{
		learningsFile: learnings.NewFile(tmpDir),
		rulesPath:     rulesPath,
	}

	tests := []struct {
		name     string
		proposal RuleChangeProposal
	}{
		{
			name: "missing current rule",
			proposal: RuleChangeProposal{
				CurrentRule:  "",
				ProposedRule: "New",
			},
		},
		{
			name: "missing proposed rule",
			proposal: RuleChangeProposal{
				CurrentRule:  "Old",
				ProposedRule: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.applyRuleChange(tt.proposal)
			if err == nil {
				t.Error("expected error for missing field")
			}
		})
	}
}
