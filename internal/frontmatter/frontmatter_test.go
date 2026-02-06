package frontmatter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWithFrontmatter(t *testing.T) {
	content := `---
id: test-spec
priority: 1
tags:
  - feature
  - important
---

# Test Document

This is the body content.
`

	fm, body, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check frontmatter
	if fm["id"] != "test-spec" {
		t.Errorf("expected id 'test-spec', got %v", fm["id"])
	}
	if fm["priority"] != 1 {
		t.Errorf("expected priority 1, got %v", fm["priority"])
	}

	tags, ok := fm["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags to be a slice, got %T", fm["tags"])
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}

	// Check body
	expectedBody := "\n# Test Document\n\nThis is the body content.\n"
	if body != expectedBody {
		t.Errorf("body mismatch\nexpected: %q\ngot: %q", expectedBody, body)
	}
}

func TestParseWithoutFrontmatter(t *testing.T) {
	content := `# Test Document

This is just body content with no frontmatter.
`

	fm, body, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter map, got %v", fm)
	}

	if body != content {
		t.Errorf("expected body to be full content\nexpected: %q\ngot: %q", content, body)
	}
}

func TestParseUnclosedFrontmatter(t *testing.T) {
	content := `---
id: test
priority: 1

# This is missing the closing delimiter
`

	_, _, err := Parse(content)
	if err == nil {
		t.Error("expected error for unclosed frontmatter, got nil")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	content := `---
id: test
invalid yaml: [not, closed
---

Body content
`

	_, _, err := Parse(content)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestSerializeWithFrontmatter(t *testing.T) {
	fm := map[string]interface{}{
		"id":       "test-spec",
		"priority": 1,
		"tags":     []string{"feature", "important"},
	}
	body := "\n# Test Document\n\nBody content.\n"

	content, err := Serialize(fm, body)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Parse it back to verify roundtrip
	fm2, body2, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse roundtrip failed: %v", err)
	}

	if fm2["id"] != "test-spec" {
		t.Errorf("roundtrip: expected id 'test-spec', got %v", fm2["id"])
	}
	if body2 != body {
		t.Errorf("roundtrip: body mismatch\nexpected: %q\ngot: %q", body, body2)
	}
}

func TestSerializeWithoutFrontmatter(t *testing.T) {
	fm := map[string]interface{}{}
	body := "# Test\n\nBody only.\n"

	content, err := Serialize(fm, body)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if content != body {
		t.Errorf("expected content to equal body\nexpected: %q\ngot: %q", body, content)
	}
}

func TestRoundtrip(t *testing.T) {
	original := `---
id: pipeline-stages
source_ideas: []
created: 2026-02-06
status: planning
---

# Pipeline Stages

## Specification

This is the spec body with **markdown** formatting.

- List item 1
- List item 2
`

	// Parse
	fm, body, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Serialize
	reconstructed, err := Serialize(fm, body)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Parse again to verify
	fm2, body2, err := Parse(reconstructed)
	if err != nil {
		t.Fatalf("Parse roundtrip failed: %v", err)
	}

	if fm2["id"] != "pipeline-stages" {
		t.Errorf("roundtrip: expected id 'pipeline-stages', got %v", fm2["id"])
	}
	if fm2["status"] != "planning" {
		t.Errorf("roundtrip: expected status 'planning', got %v", fm2["status"])
	}
	if body2 != body {
		t.Errorf("roundtrip: body changed")
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := `---
id: test-doc
version: 1
---

# Test

Body content here.
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fm, body, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if fm["id"] != "test-doc" {
		t.Errorf("expected id 'test-doc', got %v", fm["id"])
	}
	if fm["version"] != 1 {
		t.Errorf("expected version 1, got %v", fm["version"])
	}

	expectedBody := "\n# Test\n\nBody content here.\n"
	if body != expectedBody {
		t.Errorf("body mismatch\nexpected: %q\ngot: %q", expectedBody, body)
	}
}

func TestReadFileNonExistent(t *testing.T) {
	_, _, err := ReadFile("/nonexistent/path/file.md")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestUpdateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	// Write initial file
	initial := `---
id: test-doc
status: draft
version: 1
---

# Test Document

Original body content.
`

	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Update frontmatter
	updates := map[string]interface{}{
		"status":  "complete",
		"version": 2,
		"updated": "2026-02-06",
	}

	if err := UpdateFile(path, updates); err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}

	// Read back and verify
	fm, body, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Check updated fields
	if fm["status"] != "complete" {
		t.Errorf("expected status 'complete', got %v", fm["status"])
	}
	if fm["version"] != 2 {
		t.Errorf("expected version 2, got %v", fm["version"])
	}
	if fm["updated"] != "2026-02-06" {
		t.Errorf("expected updated '2026-02-06', got %v", fm["updated"])
	}

	// Check preserved fields
	if fm["id"] != "test-doc" {
		t.Errorf("expected id 'test-doc' to be preserved, got %v", fm["id"])
	}

	// Check body unchanged
	expectedBody := "\n# Test Document\n\nOriginal body content.\n"
	if body != expectedBody {
		t.Errorf("body should be unchanged\nexpected: %q\ngot: %q", expectedBody, body)
	}
}

func TestUpdateFileWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	// Write file without frontmatter
	initial := `# Test Document

Just body content, no frontmatter.
`

	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Add frontmatter via update
	updates := map[string]interface{}{
		"id":      "new-doc",
		"created": "2026-02-06",
	}

	if err := UpdateFile(path, updates); err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}

	// Read back and verify
	fm, body, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if fm["id"] != "new-doc" {
		t.Errorf("expected id 'new-doc', got %v", fm["id"])
	}
	if fm["created"] != "2026-02-06" {
		t.Errorf("expected created '2026-02-06', got %v", fm["created"])
	}

	// Body should be unchanged
	if body != initial {
		t.Errorf("body should be unchanged\nexpected: %q\ngot: %q", initial, body)
	}
}

func TestUpdateFileNonExistent(t *testing.T) {
	updates := map[string]interface{}{"status": "complete"}
	err := UpdateFile("/nonexistent/path/file.md", updates)
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}
