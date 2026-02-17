package backlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCategorizeIdea(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		// Bug keywords
		{"Fix the auth bug", "bug"},
		{"Something is broken in checkout", "bug"},
		{"Error when submitting form", "bug"},
		{"App crashes on startup", "bug"},

		// Feature keywords
		{"Add cost tracking", "feature"},
		{"Implement dark mode", "feature"},
		{"Create new dashboard", "feature"},
		{"New feature for exports", "feature"},

		// Chore keywords
		{"Refactor the auth layer", "chore"},
		{"Clean up old tests", "chore"},
		{"Update dependencies", "chore"},
		{"Upgrade to Go 1.23", "chore"},

		// Unknown
		{"What if we had a web dashboard?", "unknown"},
		{"Think about the architecture", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := CategorizeIdea(tt.text)
			if result != tt.expected {
				t.Errorf("CategorizeIdea(%q) = %q, want %q", tt.text, result, tt.expected)
			}
		})
	}
}

func TestBacklogFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	bf, _ := NewFile(tmpDir)

	// Test adding ideas
	idea1 := &Idea{
		ID:        GenerateID(),
		Text:      "Add cost tracking",
		Type:      "feature",
		Context:   "Track API costs per user",
		CreatedAt: time.Now(),
	}

	if err := bf.Add(idea1); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	idea2 := &Idea{
		ID:        GenerateID(),
		Text:      "Fix auth bug",
		Type:      "bug",
		Context:   "",
		CreatedAt: time.Now(),
	}

	if err := bf.Add(idea2); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Test listing ideas
	ideas, err := bf.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ideas) != 2 {
		t.Errorf("List() returned %d ideas, want 2", len(ideas))
	}

	// Verify content
	if ideas[0].Text != idea1.Text {
		t.Errorf("First idea text = %q, want %q", ideas[0].Text, idea1.Text)
	}
	if ideas[0].Type != idea1.Type {
		t.Errorf("First idea type = %q, want %q", ideas[0].Type, idea1.Type)
	}

	// Test list on non-existent file returns empty slice (not nil)
	emptyBf, _ := NewFile(filepath.Join(tmpDir, "nonexistent"))
	emptyIdeas, err := emptyBf.List()
	if err != nil {
		t.Fatalf("List() on non-existent file error = %v", err)
	}
	if emptyIdeas == nil {
		t.Error("List() on non-existent file returned nil, want empty slice")
	}
	if len(emptyIdeas) != 0 {
		t.Errorf("List() on non-existent file returned %d ideas, want 0", len(emptyIdeas))
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := id1
	deadline := time.Now().Add(25 * time.Millisecond)
	for id2 == id1 && time.Now().Before(deadline) {
		id2 = GenerateID()
	}

	if id1 == id2 {
		t.Errorf("GenerateID() produced duplicate IDs: %s", id1)
	}

	if len(id1) == 0 {
		t.Error("GenerateID() produced empty ID")
	}
}

func TestBacklogFileDelete(t *testing.T) {
	tmpDir := t.TempDir()
	bf, _ := NewFile(tmpDir)

	// Add three ideas
	idea1 := &Idea{ID: "idea-1", Text: "First idea", Type: "feature", CreatedAt: time.Now()}
	idea2 := &Idea{ID: "idea-2", Text: "Second idea", Type: "bug", CreatedAt: time.Now()}
	idea3 := &Idea{ID: "idea-3", Text: "Third idea", Type: "chore", CreatedAt: time.Now()}

	for _, idea := range []*Idea{idea1, idea2, idea3} {
		if err := bf.Add(idea); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Delete middle idea
	if err := bf.Delete("idea-2"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify remaining ideas
	ideas, err := bf.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ideas) != 2 {
		t.Fatalf("List() returned %d ideas, want 2", len(ideas))
	}
	if ideas[0].ID != "idea-1" {
		t.Errorf("First idea ID = %q, want %q", ideas[0].ID, "idea-1")
	}
	if ideas[1].ID != "idea-3" {
		t.Errorf("Second idea ID = %q, want %q", ideas[1].ID, "idea-3")
	}

	// Deleting non-existent idea should error
	if err := bf.Delete("idea-999"); err == nil {
		t.Error("Delete() on non-existent idea should return error")
	}
}

func TestBacklogFileGet(t *testing.T) {
	tmpDir := t.TempDir()
	bf, _ := NewFile(tmpDir)

	// Add ideas
	idea1 := &Idea{ID: "idea-1", Text: "First idea", Type: "feature", CreatedAt: time.Now()}
	idea2 := &Idea{ID: "idea-2", Text: "Second idea", Type: "bug", CreatedAt: time.Now()}

	for _, idea := range []*Idea{idea1, idea2} {
		if err := bf.Add(idea); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Get existing idea
	got, err := bf.Get("idea-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil for existing idea")
	}
	if got.Text != "First idea" {
		t.Errorf("Get() text = %q, want %q", got.Text, "First idea")
	}

	// Get non-existent idea
	got, err = bf.Get("idea-999")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != nil {
		t.Errorf("Get() returned non-nil for non-existent idea: %v", got)
	}

	// Get from empty backlog
	emptyBf, _ := NewFile(filepath.Join(tmpDir, "nonexistent"))
	got, err = emptyBf.Get("idea-1")
	if err != nil {
		t.Fatalf("Get() on empty backlog error = %v", err)
	}
	if got != nil {
		t.Errorf("Get() on empty backlog returned non-nil: %v", got)
	}
}

func TestBacklogFileListEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	bf, _ := NewFile(tmpDir)

	// Create an empty backlog file
	emptyFile := filepath.Join(tmpDir, "backlog.jsonl")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("creating empty file: %v", err)
	}

	// List should return empty slice, not nil
	ideas, err := bf.List()
	if err != nil {
		t.Fatalf("List() on empty file error = %v", err)
	}
	if ideas == nil {
		t.Error("List() on empty file returned nil, want empty slice")
	}
	if len(ideas) != 0 {
		t.Errorf("List() on empty file returned %d ideas, want 0", len(ideas))
	}
}

func TestBacklogFileCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir")

	bf, _ := NewFile(nestedPath)

	idea := &Idea{
		ID:        GenerateID(),
		Text:      "Test idea",
		Type:      "feature",
		CreatedAt: time.Now(),
	}

	if err := bf.Add(idea); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Add() did not create nested directory")
	}
}

func TestBacklogFileUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	bf, _ := NewFile(tmpDir)

	// Add test ideas
	idea1 := &Idea{ID: "idea-1", Text: "First idea", Type: "feature", CreatedAt: time.Now()}
	idea2 := &Idea{ID: "idea-2", Text: "Second idea", Type: "bug", CreatedAt: time.Now()}

	for _, idea := range []*Idea{idea1, idea2} {
		if err := bf.Add(idea); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Update first idea with status and spec name
	err := bf.Update("idea-1", func(idea *Idea) {
		idea.Status = "refined"
		idea.SpecName = "auth-system"
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	updated, err := bf.Get("idea-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.Status != "refined" {
		t.Errorf("Updated idea status = %q, want %q", updated.Status, "refined")
	}
	if updated.SpecName != "auth-system" {
		t.Errorf("Updated idea spec_name = %q, want %q", updated.SpecName, "auth-system")
	}

	// Verify other idea unchanged
	unchanged, err := bf.Get("idea-2")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if unchanged.Status != "" {
		t.Errorf("Unchanged idea status = %q, want empty", unchanged.Status)
	}

	// Update non-existent idea should error
	err = bf.Update("idea-999", func(idea *Idea) {
		idea.Status = "refined"
	})
	if err == nil {
		t.Error("Update() on non-existent idea should return error")
	}
}

func TestIdeaFieldsRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	bf, _ := NewFile(tmpDir)

	// Create idea with new fields
	original := &Idea{
		ID:        "idea-1",
		Text:      "Test idea",
		Type:      "feature",
		Context:   "Some context",
		CreatedAt: time.Now().Truncate(time.Second), // Truncate for comparison
		Status:    "refined",
		SpecName:  "test-spec",
	}

	if err := bf.Add(original); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Read back
	retrieved, err := bf.Get("idea-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify all fields roundtrip correctly
	if retrieved.ID != original.ID {
		t.Errorf("ID = %q, want %q", retrieved.ID, original.ID)
	}
	if retrieved.Text != original.Text {
		t.Errorf("Text = %q, want %q", retrieved.Text, original.Text)
	}
	if retrieved.Type != original.Type {
		t.Errorf("Type = %q, want %q", retrieved.Type, original.Type)
	}
	if retrieved.Context != original.Context {
		t.Errorf("Context = %q, want %q", retrieved.Context, original.Context)
	}
	if !retrieved.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", retrieved.CreatedAt, original.CreatedAt)
	}
	if retrieved.Status != original.Status {
		t.Errorf("Status = %q, want %q", retrieved.Status, original.Status)
	}
	if retrieved.SpecName != original.SpecName {
		t.Errorf("SpecName = %q, want %q", retrieved.SpecName, original.SpecName)
	}
}

func TestBackwardsCompatibilityWithExistingEntries(t *testing.T) {
	tmpDir := t.TempDir()
	bf, _ := NewFile(tmpDir)

	// Manually write an old-format entry without Status/SpecName fields
	oldFormatJSON := `{"id":"idea-old","text":"Old format idea","type":"feature","context":"","created_at":"2026-01-01T00:00:00Z"}` + "\n"
	backlogPath := filepath.Join(tmpDir, "backlog.jsonl")
	if err := os.WriteFile(backlogPath, []byte(oldFormatJSON), 0644); err != nil {
		t.Fatalf("writing old format entry: %v", err)
	}

	// Read it back - should not error
	ideas, err := bf.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ideas) != 1 {
		t.Fatalf("List() returned %d ideas, want 1", len(ideas))
	}

	// Verify old fields present, new fields empty
	idea := ideas[0]
	if idea.ID != "idea-old" {
		t.Errorf("ID = %q, want %q", idea.ID, "idea-old")
	}
	if idea.Text != "Old format idea" {
		t.Errorf("Text = %q, want %q", idea.Text, "Old format idea")
	}
	if idea.Status != "" {
		t.Errorf("Status = %q, want empty string", idea.Status)
	}
	if idea.SpecName != "" {
		t.Errorf("SpecName = %q, want empty string", idea.SpecName)
	}

	// Add a new entry with new fields - should coexist
	newIdea := &Idea{
		ID:        "idea-new",
		Text:      "New format idea",
		Type:      "feature",
		CreatedAt: time.Now(),
		Status:    "refined",
		SpecName:  "my-spec",
	}

	if err := bf.Add(newIdea); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// List both
	allIdeas, err := bf.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(allIdeas) != 2 {
		t.Fatalf("List() returned %d ideas, want 2", len(allIdeas))
	}

	// Verify both entries are valid
	if allIdeas[0].ID != "idea-old" || allIdeas[1].ID != "idea-new" {
		t.Error("Mixed format entries not preserved correctly")
	}
}
