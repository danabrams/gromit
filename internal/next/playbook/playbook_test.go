package playbook

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEntry_Creation(t *testing.T) {
	entry := Entry{
		ID:               "pb-12345678",
		Type:             "pattern",
		Title:            "Test Entry",
		Content:          "Some content",
		Rationale:        "Why this matters",
		Status:           "active",
		SourceProposalID: "prop-001",
		SourceRunID:      "run-001",
		SourceSpecID:     "spec-001",
		CreatedAt:        time.Now(),
		SupersededBy:     "",
	}

	if entry.ID != "pb-12345678" {
		t.Errorf("ID not set correctly: got %s", entry.ID)
	}
	if entry.Type != "pattern" {
		t.Errorf("Type not set correctly: got %s", entry.Type)
	}
	if entry.Status != "active" {
		t.Errorf("Status not set correctly: got %s", entry.Status)
	}
}

func TestComputeID(t *testing.T) {
	tests := []struct {
		name       string
		typ        string
		content    string
		wantPrefix string
	}{
		{
			name:       "basic computation",
			typ:        "pattern",
			content:    "test content",
			wantPrefix: "pb-",
		},
		{
			name:       "normalizes whitespace",
			typ:        "pattern",
			content:    "test  content  with   spaces",
			wantPrefix: "pb-",
		},
		{
			name:       "same type and content produces same ID",
			typ:        "pattern",
			content:    "same content",
			wantPrefix: "pb-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := ComputeID(tt.typ, tt.content)

			if len(id) != len("pb-")+8 {
				t.Errorf("ID length wrong: got %d chars, want %d", len(id), len("pb-")+8)
			}

			if id[:3] != tt.wantPrefix {
				t.Errorf("ID prefix wrong: got %s, want %s", id[:3], tt.wantPrefix)
			}

			// Verify deterministic
			id2 := ComputeID(tt.typ, tt.content)
			if id != id2 {
				t.Errorf("ComputeID not deterministic: %s != %s", id, id2)
			}
		})
	}
}

func TestComputeID_Deterministic(t *testing.T) {
	typ := "pattern"
	content := "test content for determinism"

	// Compute ID multiple times
	id1 := ComputeID(typ, content)
	id2 := ComputeID(typ, content)
	id3 := ComputeID(typ, content)

	if id1 != id2 || id2 != id3 {
		t.Errorf("ComputeID not deterministic: id1=%s, id2=%s, id3=%s", id1, id2, id3)
	}

	// Verify format: pb- prefix and 8 hex chars
	if len(id1) != 11 { // len("pb-") + 8
		t.Errorf("ID length wrong: got %d chars, want 11", len(id1))
	}
	if id1[:3] != "pb-" {
		t.Errorf("ID prefix wrong: got %s, want pb-", id1[:3])
	}

	// Verify hex chars (0-9, a-f)
	for i := 3; i < len(id1); i++ {
		ch := id1[i]
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("ID contains non-hex character at position %d: %c", i, ch)
		}
	}
}

func TestComputeID_DifferentInputsProduceDifferentOutput(t *testing.T) {
	// Different type should produce different ID
	id1 := ComputeID("pattern", "same content")
	id2 := ComputeID("insight", "same content")
	if id1 == id2 {
		t.Errorf("Different types produced same ID: %s", id1)
	}

	// Different content should produce different ID
	id3 := ComputeID("pattern", "content one")
	id4 := ComputeID("pattern", "content two")
	if id3 == id4 {
		t.Errorf("Different content produced same ID: %s", id3)
	}
}

func TestComputeID_WhitespaceNormalization(t *testing.T) {
	// Different whitespace, same normalized content should produce same ID
	id1 := ComputeID("pattern", "hello   world")
	id2 := ComputeID("pattern", "hello world")
	id3 := ComputeID("pattern", "hello\n\nworld")
	id4 := ComputeID("pattern", "  hello world  ") // leading/trailing whitespace
	id5 := ComputeID("pattern", "\thello\t\t\tworld\t")

	if id1 != id2 {
		t.Errorf("Multiple spaces not normalized: %s != %s", id1, id2)
	}
	if id1 != id3 {
		t.Errorf("Newlines not normalized: %s != %s", id1, id3)
	}
	if id1 != id4 {
		t.Errorf("Leading/trailing whitespace not trimmed: %s != %s", id1, id4)
	}
	if id1 != id5 {
		t.Errorf("Tabs not normalized: %s != %s", id1, id5)
	}
}

func TestComputeEntryID(t *testing.T) {
	// Test ComputeID function directly with entry data
	tests := []struct {
		name                string
		typ                 string
		content             string
		expectDeterministic bool
	}{
		{
			name:                "compute entry ID for pattern",
			typ:                 "pattern",
			content:             "Always prefer simple solutions over complex ones",
			expectDeterministic: true,
		},
		{
			name:                "compute entry ID for insight",
			typ:                 "insight",
			content:             "Documentation reduces debugging time by 40%",
			expectDeterministic: true,
		},
		{
			name:                "compute entry ID for decision",
			typ:                 "decision",
			content:             "Use PostgreSQL for persistence",
			expectDeterministic: true,
		},
		{
			name:                "whitespace normalization before hashing",
			typ:                 "pattern",
			content:             "Same   content  with\n\nvarious   whitespace",
			expectDeterministic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := ComputeID(tt.typ, tt.content)

			// Verify ID format: pb- prefix followed by 8 hex characters
			if len(id) != 11 {
				t.Errorf("ID length: got %d, want 11", len(id))
			}

			if id[:3] != "pb-" {
				t.Errorf("ID prefix: got %s, want pb-", id[:3])
			}

			// Verify hex characters
			for i := 3; i < len(id); i++ {
				ch := id[i]
				if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
					t.Errorf("ID contains non-hex character at position %d: %c", i, ch)
				}
			}

			// Verify determinism
			if tt.expectDeterministic {
				id2 := ComputeID(tt.typ, tt.content)
				if id != id2 {
					t.Errorf("ID not deterministic: %s != %s", id, id2)
				}
			}
		})
	}
}

func TestStore_Save_And_Load(t *testing.T) {
	tmpDir := t.TempDir()
	store := Store{Dir: tmpDir}

	// Create test entries
	now := time.Now()
	entries := []Entry{
		{
			ID:               "pb-11111111",
			Type:             "pattern",
			Title:            "Entry 1",
			Content:          "Content 1",
			Rationale:        "Rationale 1",
			Status:           "active",
			SourceProposalID: "prop-001",
			SourceRunID:      "run-001",
			SourceSpecID:     "spec-001",
			CreatedAt:        now,
			SupersededBy:     "",
		},
		{
			ID:               "pb-22222222",
			Type:             "insight",
			Title:            "Entry 2",
			Content:          "Content 2",
			Rationale:        "Rationale 2",
			Status:           "archived",
			SourceProposalID: "prop-002",
			SourceRunID:      "run-002",
			SourceSpecID:     "spec-002",
			CreatedAt:        now.Add(1 * time.Hour),
			SupersededBy:     "pb-11111111",
		},
	}

	// Save
	err := store.Save(entries)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, "entries.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("entries.json not created: %v", err)
	}

	// Load
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded) != len(entries) {
		t.Errorf("Loaded entries count: got %d, want %d", len(loaded), len(entries))
	}

	// Verify content
	for i, entry := range entries {
		if i >= len(loaded) {
			break
		}
		if loaded[i].ID != entry.ID {
			t.Errorf("Entry %d ID: got %s, want %s", i, loaded[i].ID, entry.ID)
		}
		if loaded[i].Type != entry.Type {
			t.Errorf("Entry %d Type: got %s, want %s", i, loaded[i].Type, entry.Type)
		}
		if loaded[i].Title != entry.Title {
			t.Errorf("Entry %d Title: got %s, want %s", i, loaded[i].Title, entry.Title)
		}
		if loaded[i].Status != entry.Status {
			t.Errorf("Entry %d Status: got %s, want %s", i, loaded[i].Status, entry.Status)
		}
	}
}

func TestStore_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store := Store{Dir: tmpDir}

	// Load from non-existent file should return empty slice, not error
	entries, err := store.Load()
	if err != nil {
		t.Fatalf("Load from non-existent file should not error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Load from non-existent file: got %d entries, want 0", len(entries))
	}
}

func TestStore_Load_CorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	store := Store{Dir: tmpDir}

	// Write corrupted JSON
	filePath := filepath.Join(tmpDir, "entries.json")
	if err := os.WriteFile(filePath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load should error
	_, err := store.Load()
	if err == nil {
		t.Error("Load from corrupted JSON should error, but got nil")
	}
}

func TestActiveEntries_Empty(t *testing.T) {
	entries := []Entry{
		{ID: "pb-11111111", Status: "archived", Title: "Archived 1"},
		{ID: "pb-22222222", Status: "inactive", Title: "Inactive 1"},
	}

	active := ActiveEntries(entries)

	if len(active) != 0 {
		t.Errorf("ActiveEntries with no active: got %d, want 0", len(active))
	}
}

func TestStore_Persistence_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	store := Store{Dir: tmpDir}

	// Create entries with various field types
	original := []Entry{
		{
			ID:               "pb-aaaaaaaa",
			Type:             "pattern",
			Title:            "Pattern 1",
			Content:          "Complex content with\nnewlines and\ttabs",
			Rationale:        "Why this is important",
			Status:           "active",
			SourceProposalID: "prop-123",
			SourceRunID:      "run-456",
			SourceSpecID:     "spec-789",
			CreatedAt:        time.Date(2026, 3, 21, 10, 30, 0, 0, time.UTC),
			SupersededBy:     "",
		},
	}

	// Save
	if err := store.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Compare
	if len(loaded) != len(original) {
		t.Fatalf("Count mismatch: got %d, want %d", len(loaded), len(original))
	}

	// Verify all fields preserved
	orig := original[0]
	load := loaded[0]

	if load.ID != orig.ID {
		t.Errorf("ID mismatch: %s != %s", load.ID, orig.ID)
	}
	if load.Type != orig.Type {
		t.Errorf("Type mismatch: %s != %s", load.Type, orig.Type)
	}
	if load.Content != orig.Content {
		t.Errorf("Content mismatch: %s != %s", load.Content, orig.Content)
	}
	if load.Status != orig.Status {
		t.Errorf("Status mismatch: %s != %s", load.Status, orig.Status)
	}
	if load.SupersededBy != orig.SupersededBy {
		t.Errorf("SupersededBy mismatch: %s != %s", load.SupersededBy, orig.SupersededBy)
	}
}

func TestStore_LoadSave_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	store := Store{Dir: tmpDir}

	// Create comprehensive test entries with all fields populated
	now := time.Date(2026, 3, 21, 14, 30, 45, 123456789, time.UTC)
	entries := []Entry{
		{
			ID:               "pb-11111111",
			Type:             "pattern",
			Title:            "Test Pattern",
			Content:          "Pattern content with special chars: !@#$%",
			Rationale:        "Pattern rationale",
			Status:           "active",
			SourceProposalID: "prop-001",
			SourceRunID:      "run-001",
			SourceSpecID:     "spec-001",
			CreatedAt:        now,
			SupersededBy:     "",
		},
		{
			ID:               "pb-22222222",
			Type:             "insight",
			Title:            "Test Insight",
			Content:          "Insight with\nmultiple\nlines",
			Rationale:        "Insight rationale",
			Status:           "archived",
			SourceProposalID: "prop-002",
			SourceRunID:      "run-002",
			SourceSpecID:     "spec-002",
			CreatedAt:        now.Add(24 * time.Hour),
			SupersededBy:     "pb-11111111",
		},
		{
			ID:               "pb-33333333",
			Type:             "decision",
			Title:            "Complex Entry",
			Content:          "Content with\ttabs\tand\nnewlines",
			Rationale:        "Complex rationale",
			Status:           "active",
			SourceProposalID: "prop-003",
			SourceRunID:      "run-003",
			SourceSpecID:     "spec-003",
			CreatedAt:        now.Add(48 * time.Hour),
			SupersededBy:     "",
		},
	}

	// Save entries
	if err := store.Save(entries); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load entries
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify count
	if len(loaded) != len(entries) {
		t.Fatalf("Entry count mismatch: got %d, want %d", len(loaded), len(entries))
	}

	// Verify all entries and fields
	for i, want := range entries {
		got := loaded[i]

		// Check all fields
		if got.ID != want.ID {
			t.Errorf("Entry %d ID: got %q, want %q", i, got.ID, want.ID)
		}
		if got.Type != want.Type {
			t.Errorf("Entry %d Type: got %q, want %q", i, got.Type, want.Type)
		}
		if got.Title != want.Title {
			t.Errorf("Entry %d Title: got %q, want %q", i, got.Title, want.Title)
		}
		if got.Content != want.Content {
			t.Errorf("Entry %d Content: got %q, want %q", i, got.Content, want.Content)
		}
		if got.Rationale != want.Rationale {
			t.Errorf("Entry %d Rationale: got %q, want %q", i, got.Rationale, want.Rationale)
		}
		if got.Status != want.Status {
			t.Errorf("Entry %d Status: got %q, want %q", i, got.Status, want.Status)
		}
		if got.SourceProposalID != want.SourceProposalID {
			t.Errorf("Entry %d SourceProposalID: got %q, want %q", i, got.SourceProposalID, want.SourceProposalID)
		}
		if got.SourceRunID != want.SourceRunID {
			t.Errorf("Entry %d SourceRunID: got %q, want %q", i, got.SourceRunID, want.SourceRunID)
		}
		if got.SourceSpecID != want.SourceSpecID {
			t.Errorf("Entry %d SourceSpecID: got %q, want %q", i, got.SourceSpecID, want.SourceSpecID)
		}
		if got.CreatedAt != want.CreatedAt {
			t.Errorf("Entry %d CreatedAt: got %v, want %v", i, got.CreatedAt, want.CreatedAt)
		}
		if got.SupersededBy != want.SupersededBy {
			t.Errorf("Entry %d SupersededBy: got %q, want %q", i, got.SupersededBy, want.SupersededBy)
		}
	}
}

func TestStore_Load_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := Store{Dir: tmpDir}

	// Load from directory without entries.json should return empty slice without error
	entries, err := store.Load()
	if err != nil {
		t.Errorf("Load from missing file should not error: %v", err)
	}

	if entries == nil {
		t.Error("Load from missing file should return empty slice, not nil")
	}

	if len(entries) != 0 {
		t.Errorf("Load from missing file: got %d entries, want 0", len(entries))
	}
}

func TestStore_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a path with nested directories that don't exist
	nestedDir := filepath.Join(tmpDir, "nested", "playbook", "data")
	store := Store{Dir: nestedDir}

	entries := []Entry{
		{
			ID:        "pb-testtest",
			Type:      "pattern",
			Title:     "Test",
			Content:   "Test content",
			Rationale: "Test rationale",
			Status:    "active",
		},
	}

	// Save should create directory and succeed
	if err := store.Save(entries); err != nil {
		t.Fatalf("Save should create directory if needed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); err != nil {
		t.Fatalf("Directory not created: %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(nestedDir, "entries.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("entries.json not created: %v", err)
	}

	// Verify we can load it back
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(loaded))
	}
}

func TestNormalizeNilFields(t *testing.T) {
	tests := []struct {
		name string
		e    Entry
	}{
		{
			name: "empty entry",
			e:    Entry{},
		},
		{
			name: "entry with all fields set",
			e: Entry{
				ID:               "pb-12345678",
				Type:             "pattern",
				Title:            "Test",
				Content:          "Content",
				Rationale:        "Rationale",
				Status:           "active",
				SourceProposalID: "prop-001",
				SourceRunID:      "run-001",
				SourceSpecID:     "spec-001",
				CreatedAt:        time.Now(),
				SupersededBy:     "",
			},
		},
		{
			name: "entry with empty string fields",
			e: Entry{
				ID:               "",
				Type:             "",
				Title:            "",
				Content:          "",
				Rationale:        "",
				Status:           "",
				SourceProposalID: "",
				SourceRunID:      "",
				SourceSpecID:     "",
				CreatedAt:        time.Time{},
				SupersededBy:     "",
			},
		},
		{
			name: "entry with zero time",
			e: Entry{
				ID:        "pb-12345678",
				Status:    "active",
				CreatedAt: time.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.e
			tt.e.NormalizeNilFields()

			// Verify entry is unchanged (NormalizeNilFields is a no-op for Entry)
			if tt.e.ID != original.ID {
				t.Errorf("ID changed: got %s, want %s", tt.e.ID, original.ID)
			}
			if tt.e.Type != original.Type {
				t.Errorf("Type changed: got %s, want %s", tt.e.Type, original.Type)
			}
			if tt.e.Title != original.Title {
				t.Errorf("Title changed: got %s, want %s", tt.e.Title, original.Title)
			}
			if tt.e.Content != original.Content {
				t.Errorf("Content changed: got %s, want %s", tt.e.Content, original.Content)
			}
			if tt.e.Rationale != original.Rationale {
				t.Errorf("Rationale changed: got %s, want %s", tt.e.Rationale, original.Rationale)
			}
			if tt.e.Status != original.Status {
				t.Errorf("Status changed: got %s, want %s", tt.e.Status, original.Status)
			}
			if tt.e.SourceProposalID != original.SourceProposalID {
				t.Errorf("SourceProposalID changed: got %s, want %s", tt.e.SourceProposalID, original.SourceProposalID)
			}
			if tt.e.SourceRunID != original.SourceRunID {
				t.Errorf("SourceRunID changed: got %s, want %s", tt.e.SourceRunID, original.SourceRunID)
			}
			if tt.e.SourceSpecID != original.SourceSpecID {
				t.Errorf("SourceSpecID changed: got %s, want %s", tt.e.SourceSpecID, original.SourceSpecID)
			}
			if tt.e.CreatedAt != original.CreatedAt {
				t.Errorf("CreatedAt changed: got %v, want %v", tt.e.CreatedAt, original.CreatedAt)
			}
			if tt.e.SupersededBy != original.SupersededBy {
				t.Errorf("SupersededBy changed: got %s, want %s", tt.e.SupersededBy, original.SupersededBy)
			}
		})
	}
}

func TestActiveEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		wantIDs []string
		wantLen int
	}{
		{
			name:    "empty list",
			entries: []Entry{},
			wantIDs: []string{},
			wantLen: 0,
		},
		{
			name: "single active entry",
			entries: []Entry{
				{
					ID:           "pb-11111111",
					Status:       "active",
					SupersededBy: "",
				},
			},
			wantIDs: []string{"pb-11111111"},
			wantLen: 1,
		},
		{
			name: "multiple active entries",
			entries: []Entry{
				{ID: "pb-11111111", Status: "active", SupersededBy: ""},
				{ID: "pb-22222222", Status: "active", SupersededBy: ""},
				{ID: "pb-33333333", Status: "active", SupersededBy: ""},
			},
			wantIDs: []string{"pb-11111111", "pb-22222222", "pb-33333333"},
			wantLen: 3,
		},
		{
			name: "active entry excludes archived",
			entries: []Entry{
				{ID: "pb-11111111", Status: "active", SupersededBy: ""},
				{ID: "pb-22222222", Status: "archived", SupersededBy: ""},
			},
			wantIDs: []string{"pb-11111111"},
			wantLen: 1,
		},
		{
			name: "active entry excludes superseded",
			entries: []Entry{
				{ID: "pb-11111111", Status: "active", SupersededBy: ""},
				{ID: "pb-22222222", Status: "active", SupersededBy: "pb-11111111"},
			},
			wantIDs: []string{"pb-11111111"},
			wantLen: 1,
		},
		{
			name: "excludes both non-active and superseded",
			entries: []Entry{
				{ID: "pb-11111111", Status: "active", SupersededBy: ""},
				{ID: "pb-22222222", Status: "active", SupersededBy: "pb-11111111"},
				{ID: "pb-33333333", Status: "archived", SupersededBy: ""},
				{ID: "pb-44444444", Status: "draft", SupersededBy: ""},
			},
			wantIDs: []string{"pb-11111111"},
			wantLen: 1,
		},
		{
			name: "mixed statuses and superseded entries",
			entries: []Entry{
				{ID: "pb-11111111", Status: "active", SupersededBy: ""},
				{ID: "pb-22222222", Status: "active", SupersededBy: ""},
				{ID: "pb-33333333", Status: "active", SupersededBy: "pb-22222222"},
				{ID: "pb-44444444", Status: "archived", SupersededBy: ""},
				{ID: "pb-55555555", Status: "archived", SupersededBy: "pb-11111111"},
			},
			wantIDs: []string{"pb-11111111", "pb-22222222"},
			wantLen: 2,
		},
		{
			name: "all archived entries returns empty",
			entries: []Entry{
				{ID: "pb-11111111", Status: "archived", SupersededBy: ""},
				{ID: "pb-22222222", Status: "archived", SupersededBy: ""},
			},
			wantIDs: []string{},
			wantLen: 0,
		},
		{
			name: "all superseded entries returns empty",
			entries: []Entry{
				{ID: "pb-11111111", Status: "active", SupersededBy: "pb-99999999"},
				{ID: "pb-22222222", Status: "active", SupersededBy: "pb-99999999"},
			},
			wantIDs: []string{},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ActiveEntries(tt.entries)

			if len(result) != tt.wantLen {
				t.Errorf("ActiveEntries length mismatch: got %d, want %d", len(result), tt.wantLen)
			}

			if len(result) != len(tt.wantIDs) {
				t.Errorf("Expected %d IDs, got %d results", len(tt.wantIDs), len(result))
			}

			// Verify result contains expected IDs in order
			for i, id := range tt.wantIDs {
				if i < len(result) && result[i].ID != id {
					t.Errorf("Result[%d]: got ID %s, want %s", i, result[i].ID, id)
				}
			}

			// Verify all results have status="active"
			for i, entry := range result {
				if entry.Status != "active" {
					t.Errorf("Result[%d]: status is %q, want %q", i, entry.Status, "active")
				}
			}

			// Verify no results are superseded
			for i, entry := range result {
				if entry.SupersededBy != "" {
					t.Errorf("Result[%d]: SupersededBy is %q, want empty string", i, entry.SupersededBy)
				}
			}
		})
	}
}
