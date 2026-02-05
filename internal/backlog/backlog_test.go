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

	bf := NewFile(tmpDir)

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

	// Test list on non-existent file
	emptyBf := NewFile(filepath.Join(tmpDir, "nonexistent"))
	emptyIdeas, err := emptyBf.List()
	if err != nil {
		t.Fatalf("List() on non-existent file error = %v", err)
	}
	if emptyIdeas != nil {
		t.Errorf("List() on non-existent file = %v, want nil", emptyIdeas)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	time.Sleep(2 * time.Millisecond)
	id2 := GenerateID()

	if id1 == id2 {
		t.Errorf("GenerateID() produced duplicate IDs: %s", id1)
	}

	if len(id1) == 0 {
		t.Error("GenerateID() produced empty ID")
	}
}

func TestBacklogFileDelete(t *testing.T) {
	tmpDir := t.TempDir()
	bf := NewFile(tmpDir)

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

func TestBacklogFileCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir")

	bf := NewFile(nestedPath)

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
