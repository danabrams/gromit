package doctrine

import (
	"testing"
	"time"
)

func TestMergedDoctrine_LocalOnlyWhenNoGlobalRulesJSON(t *testing.T) {
	globalDir := t.TempDir() // no rules.json written — simulates absent global config
	localDir := t.TempDir()

	localStore := &FSStore{Dir: localDir}
	localRules := Doctrine{
		Rules: []Rule{
			{ID: "l1", Summary: "Local only rule 1", Scope: "*", Status: "active", CreatedAt: time.Now()},
			{ID: "l2", Summary: "Local only rule 2", Scope: "api", Status: "active", CreatedAt: time.Now()},
		},
	}
	if err := localStore.Save(localRules); err != nil {
		t.Fatalf("Save local rules: %v", err)
	}

	merged, err := MergedDoctrine(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedDoctrine: %v", err)
	}

	if len(merged) != 2 {
		t.Errorf("expected 2 rules, got %d", len(merged))
	}

	ids := make(map[string]bool)
	for _, rule := range merged {
		ids[rule.ID] = true
	}
	if !ids["l1"] {
		t.Error("l1 should be present")
	}
	if !ids["l2"] {
		t.Error("l2 should be present")
	}
}

func TestMergedDoctrine_GlobalRulesWhenNoLocal(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Setup global rules
	globalStore := &FSStore{Dir: globalDir}
	globalRules := Doctrine{
		Rules: []Rule{
			{ID: "g1", Summary: "Global rule 1", Scope: "*", Status: "active", CreatedAt: time.Now()},
			{ID: "g2", Summary: "Global rule 2", Scope: "api", Status: "active", CreatedAt: time.Now()},
		},
	}
	if err := globalStore.Save(globalRules); err != nil {
		t.Fatalf("Save global rules: %v", err)
	}

	// Local dir is empty
	merged, err := MergedDoctrine(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedDoctrine: %v", err)
	}

	if len(merged) != 2 {
		t.Errorf("expected 2 rules, got %d", len(merged))
	}

	// Verify global rules appear
	if len(merged) >= 1 && merged[0].ID != "g1" {
		t.Errorf("rule[0].ID = %q, want %q", merged[0].ID, "g1")
	}
	if len(merged) >= 2 && merged[1].ID != "g2" {
		t.Errorf("rule[1].ID = %q, want %q", merged[1].ID, "g2")
	}
}

func TestMergedDoctrine_LocalWinsSameID(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Setup global rules
	globalStore := &FSStore{Dir: globalDir}
	globalRules := Doctrine{
		Rules: []Rule{
			{ID: "r1", Summary: "Global summary", Scope: "*", Status: "active", CreatedAt: time.Now()},
		},
	}
	if err := globalStore.Save(globalRules); err != nil {
		t.Fatalf("Save global: %v", err)
	}

	// Setup local rules with same ID but different summary
	localStore := &FSStore{Dir: localDir}
	localRules := Doctrine{
		Rules: []Rule{
			{ID: "r1", Summary: "Local summary", Scope: "api", Status: "active", CreatedAt: time.Now()},
		},
	}
	if err := localStore.Save(localRules); err != nil {
		t.Fatalf("Save local: %v", err)
	}

	merged, err := MergedDoctrine(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedDoctrine: %v", err)
	}

	if len(merged) != 1 {
		t.Errorf("expected 1 rule, got %d", len(merged))
	}

	if merged[0].Summary != "Local summary" {
		t.Errorf("Summary = %q, want %q", merged[0].Summary, "Local summary")
	}
	if merged[0].Scope != "api" {
		t.Errorf("Scope = %q, want %q", merged[0].Scope, "api")
	}
}

func TestMergedDoctrine_SupersededMasksGlobal(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Setup global rules
	globalStore := &FSStore{Dir: globalDir}
	globalRules := Doctrine{
		Rules: []Rule{
			{ID: "r1", Summary: "Old rule", Scope: "*", Status: "active", CreatedAt: time.Now()},
			{ID: "r2", Summary: "Other rule", Scope: "api", Status: "active", CreatedAt: time.Now()},
		},
	}
	if err := globalStore.Save(globalRules); err != nil {
		t.Fatalf("Save global: %v", err)
	}

	// Setup local rule that supersedes r1
	localStore := &FSStore{Dir: localDir}
	localRules := Doctrine{
		Rules: []Rule{
			{ID: "r1", Summary: "Old rule", Scope: "*", Status: "superseded", SupersededBy: "r3", CreatedAt: time.Now()},
			{ID: "r3", Summary: "New rule", Scope: "*", Status: "active", CreatedAt: time.Now()},
		},
	}
	if err := localStore.Save(localRules); err != nil {
		t.Fatalf("Save local: %v", err)
	}

	merged, err := MergedDoctrine(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedDoctrine: %v", err)
	}

	// Should have r2 and r3 (r1 is masked out by superseded local rule)
	if len(merged) != 2 {
		t.Errorf("expected 2 rules, got %d", len(merged))
	}

	ids := make(map[string]bool)
	for _, rule := range merged {
		ids[rule.ID] = true
	}

	if !ids["r2"] {
		t.Error("r2 should be present")
	}
	if !ids["r3"] {
		t.Error("r3 should be present")
	}
	if ids["r1"] {
		t.Error("r1 should be masked by superseded local rule")
	}
}
