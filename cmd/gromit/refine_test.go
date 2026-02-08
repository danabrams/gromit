package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSpecTitle(t *testing.T) {
	// Create a temp directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		want     string
		filename string
	}{
		{
			name: "simple heading",
			content: `# My Spec Title

This is the body.`,
			want:     "My Spec Title",
			filename: "simple.md",
		},
		{
			name: "heading with frontmatter",
			content: `---
id: test-spec
created: 2026-02-07
---

# Spec With Frontmatter

Body text here.`,
			want:     "Spec With Frontmatter",
			filename: "with-frontmatter.md",
		},
		{
			name: "heading with extra whitespace",
			content: `#    Title With Spaces

Body.`,
			want:     "Title With Spaces",
			filename: "whitespace.md",
		},
		{
			name: "only level-2 heading",
			content: `## Not Level One

Body.`,
			want:     "",
			filename: "level2.md",
		},
		{
			name:     "empty file",
			content:  ``,
			want:     "",
			filename: "empty.md",
		},
		{
			name: "no heading",
			content: `This is just text.

No heading here.`,
			want:     "",
			filename: "no-heading.md",
		},
		{
			name: "heading after other content",
			content: `Some intro text.

# The Title

Body.`,
			want:     "The Title",
			filename: "heading-after-text.md",
		},
		{
			name: "multiple headings",
			content: `# First Heading

Content.

# Second Heading

More content.`,
			want:     "First Heading",
			filename: "multiple.md",
		},
		{
			name: "frontmatter only",
			content: `---
id: test
---`,
			want:     "",
			filename: "frontmatter-only.md",
		},
		{
			name: "frontmatter with no closing",
			content: `---
id: test
key: value

# Title After Incomplete Frontmatter`,
			want:     "",
			filename: "bad-frontmatter.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			got := extractSpecTitle(filePath)
			if got != tt.want {
				t.Errorf("extractSpecTitle() = %q, want %q", got, tt.want)
			}
		})
	}

	// Test missing file
	t.Run("missing file", func(t *testing.T) {
		got := extractSpecTitle(filepath.Join(tmpDir, "nonexistent.md"))
		if got != "" {
			t.Errorf("extractSpecTitle() for missing file = %q, want empty string", got)
		}
	})

	// Test file with insufficient permissions (unreadable after open on some systems)
	// Note: This test may not reliably trigger scanner.Err() on all systems, but it documents
	// the intent to handle scanner read errors gracefully.
	t.Run("unreadable file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("skipping permission test when running as root")
		}
		// Create a readable file, then remove read permissions
		filePath := filepath.Join(tmpDir, "unreadable.md")
		if err := os.WriteFile(filePath, []byte("# Test"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
		if err := os.Chmod(filePath, 0000); err != nil {
			t.Fatalf("failed to chmod test file: %v", err)
		}
		defer os.Chmod(filePath, 0644) // restore permissions for cleanup

		got := extractSpecTitle(filePath)
		if got != "" {
			t.Errorf("extractSpecTitle() for unreadable file = %q, want empty string", got)
		}
	})
}

func TestFormatTypeLabel(t *testing.T) {
	tests := []struct {
		name     string
		ideaType string
		want     string
	}{
		{
			name:     "feature type",
			ideaType: "feature",
			want:     "[feature]",
		},
		{
			name:     "bug type",
			ideaType: "bug",
			want:     "[bug]    ",
		},
		{
			name:     "chore type",
			ideaType: "chore",
			want:     "[chore]  ",
		},
		{
			name:     "unknown type",
			ideaType: "unknown",
			want:     "[unknown]",
		},
		{
			name:     "custom type - short",
			ideaType: "docs",
			want:     "[docs   ]",
		},
		{
			name:     "custom type - exactly 7 chars",
			ideaType: "hotfix",
			want:     "[hotfix ]",
		},
		{
			name:     "custom type - longer than 7 chars",
			ideaType: "enhancement",
			want:     "[enhancement]",
		},
		{
			name:     "custom type - single char",
			ideaType: "x",
			want:     "[x      ]",
		},
		{
			name:     "empty type",
			ideaType: "",
			want:     "[       ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTypeLabel(tt.ideaType)
			if got != tt.want {
				t.Errorf("formatTypeLabel(%q) = %q, want %q", tt.ideaType, got, tt.want)
			}
		})
	}
}
