package playbook

import (
	"testing"
	"time"
)

func TestMergedPlaybook_GlobalEntriesOnlyWhenNoLocal(t *testing.T) {
	// Create temp directories for global and local
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Create global entries
	globalStore := Store{Dir: globalDir}
	globalEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Global Pattern",
			Content:   "Global content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
		{
			ID:        "pb-22222222",
			Type:      "insight",
			Title:     "Global Insight",
			Content:   "Global insight content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("Failed to save global entries: %v", err)
	}

	// Local directory is empty (no entries.json created yet)

	// Call MergedPlaybook
	merged, err := MergedPlaybook(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedPlaybook failed: %v", err)
	}

	// Should return global entries when no local entries exist
	if len(merged) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(merged))
	}

	// Verify the entries are from global
	if merged[0].ID != "pb-11111111" || merged[0].Title != "Global Pattern" {
		t.Errorf("Expected first entry to be global pattern, got ID=%s Title=%s", merged[0].ID, merged[0].Title)
	}

	if merged[1].ID != "pb-22222222" || merged[1].Title != "Global Insight" {
		t.Errorf("Expected second entry to be global insight, got ID=%s Title=%s", merged[1].ID, merged[1].Title)
	}
}

func TestMergedPlaybook_LocalOverridesGlobal(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Create global entries
	globalStore := Store{Dir: globalDir}
	globalEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Global Pattern",
			Content:   "Global content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("Failed to save global entries: %v", err)
	}

	// Create local entries with same ID but different content (should override)
	localStore := Store{Dir: localDir}
	localEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Local Override",
			Content:   "Local content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := localStore.Save(localEntries); err != nil {
		t.Fatalf("Failed to save local entries: %v", err)
	}

	// Call MergedPlaybook
	merged, err := MergedPlaybook(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedPlaybook failed: %v", err)
	}

	// Should have one entry, but it should be the local version
	if len(merged) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(merged))
	}

	if merged[0].Title != "Local Override" {
		t.Errorf("Expected local override, got %s", merged[0].Title)
	}
}

func TestMergedPlaybook_LocalSupersededMasksGlobal(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Create global entries
	globalStore := Store{Dir: globalDir}
	globalEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Global Pattern",
			Content:   "Global content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("Failed to save global entries: %v", err)
	}

	// Create local entry with Status="superseded" for same ID
	// This should mask (hide) the global entry
	localStore := Store{Dir: localDir}
	localEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Local Superseded",
			Content:   "Local content",
			Status:    "superseded",
			CreatedAt: time.Now(),
		},
	}
	if err := localStore.Save(localEntries); err != nil {
		t.Fatalf("Failed to save local entries: %v", err)
	}

	// Call MergedPlaybook
	merged, err := MergedPlaybook(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedPlaybook failed: %v", err)
	}

	// Should be empty - the global entry is masked by local superseded
	if len(merged) != 0 {
		t.Errorf("Expected 0 entries (masked by superseded), got %d", len(merged))
	}
}

func TestMergedPlaybook_NewLocalEntries(t *testing.T) {
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Create global entry
	globalStore := Store{Dir: globalDir}
	globalEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Global Pattern",
			Content:   "Global content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("Failed to save global entries: %v", err)
	}

	// Create local entries - one matches global, one is new
	localStore := Store{Dir: localDir}
	localEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Local Override",
			Content:   "Local content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
		{
			ID:        "pb-33333333",
			Type:      "insight",
			Title:     "New Local Entry",
			Content:   "New local content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := localStore.Save(localEntries); err != nil {
		t.Fatalf("Failed to save local entries: %v", err)
	}

	// Call MergedPlaybook
	merged, err := MergedPlaybook(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedPlaybook failed: %v", err)
	}

	// Should have 2 entries: the overridden one and the new local one
	if len(merged) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(merged))
	}

	// First should be the overridden global entry
	if merged[0].ID != "pb-11111111" || merged[0].Title != "Local Override" {
		t.Errorf("Expected first entry to be local override, got ID=%s Title=%s", merged[0].ID, merged[0].Title)
	}

	// Second should be the new local entry
	if merged[1].ID != "pb-33333333" || merged[1].Title != "New Local Entry" {
		t.Errorf("Expected second entry to be new local, got ID=%s Title=%s", merged[1].ID, merged[1].Title)
	}
}

func TestMergedPlaybook_NewProjectInheritsAllGlobalEntries(t *testing.T) {
	// Create temp directories for global and local
	globalDir := t.TempDir()
	localDir := t.TempDir()

	// Create global entries: 2 planner_heuristic + 1 validation_gap
	globalStore := Store{Dir: globalDir}
	globalEntries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "planner_heuristic",
			Title:     "Global Heuristic 1",
			Content:   "Heuristic content 1",
			Status:    "active",
			CreatedAt: time.Now(),
		},
		{
			ID:        "pb-22222222",
			Type:      "planner_heuristic",
			Title:     "Global Heuristic 2",
			Content:   "Heuristic content 2",
			Status:    "active",
			CreatedAt: time.Now(),
		},
		{
			ID:        "pb-33333333",
			Type:      "validation_gap",
			Title:     "Global Validation Gap",
			Content:   "Validation gap content",
			Status:    "active",
			CreatedAt: time.Now(),
		},
	}
	if err := globalStore.Save(globalEntries); err != nil {
		t.Fatalf("Failed to save global entries: %v", err)
	}

	// Local directory is empty (no entries.json created yet)

	// Call MergedPlaybook
	merged, err := MergedPlaybook(globalDir, localDir)
	if err != nil {
		t.Fatalf("MergedPlaybook failed: %v", err)
	}

	// Should return all 3 global entries when no local entries exist
	if len(merged) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(merged))
	}

	// Verify the entries are from global
	if merged[0].ID != "pb-11111111" || merged[0].Type != "planner_heuristic" || merged[0].Title != "Global Heuristic 1" {
		t.Errorf("Expected first entry to be global heuristic 1, got ID=%s Type=%s Title=%s", merged[0].ID, merged[0].Type, merged[0].Title)
	}

	if merged[1].ID != "pb-22222222" || merged[1].Type != "planner_heuristic" || merged[1].Title != "Global Heuristic 2" {
		t.Errorf("Expected second entry to be global heuristic 2, got ID=%s Type=%s Title=%s", merged[1].ID, merged[1].Type, merged[1].Title)
	}

	if merged[2].ID != "pb-33333333" || merged[2].Type != "validation_gap" || merged[2].Title != "Global Validation Gap" {
		t.Errorf("Expected third entry to be global validation gap, got ID=%s Type=%s Title=%s", merged[2].ID, merged[2].Type, merged[2].Title)
	}
}
