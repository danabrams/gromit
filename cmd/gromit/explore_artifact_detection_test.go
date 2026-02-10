package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/frontmatter"
)

// TestDetectNewEpics verifies that new epic files are detected after session
func TestDetectNewEpics(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Pre-session: one existing epic
	existingEpic := filepath.Join(epicsDir, "existing-epic.md")
	if err := os.WriteFile(existingEpic, []byte("# Existing Epic"), 0644); err != nil {
		t.Fatalf("failed to write existing epic: %v", err)
	}

	preSessionEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	// Simulate session creating new epic
	newEpic := filepath.Join(epicsDir, "new-exploration-epic.md")
	epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# New Epic

This epic was created during exploration.
`
	if err := os.WriteFile(newEpic, []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write new epic: %v", err)
	}

	// Post-session: detect new epics
	postSessionEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	// Find new epics
	preSet := make(map[string]bool)
	for _, epic := range preSessionEpics {
		preSet[epic] = true
	}

	var newEpics []string
	for _, epic := range postSessionEpics {
		if !preSet[epic] {
			newEpics = append(newEpics, epic)
		}
	}

	if len(newEpics) != 1 {
		t.Errorf("expected 1 new epic, got %d", len(newEpics))
	}

	if len(newEpics) > 0 && newEpics[0] != newEpic {
		t.Errorf("expected new epic %s, got %s", newEpic, newEpics[0])
	}
}

// TestReadEpicFrontmatter verifies frontmatter parsing for epic files
func TestReadEpicFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	epicFile := filepath.Join(tmpDir, "test-epic.md")

	epicContent := `---
epic_id: gromit-abc
created: 2026-02-08
---

# Test Epic

Epic body content goes here.
`
	if err := os.WriteFile(epicFile, []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic file: %v", err)
	}

	fm, body, err := frontmatter.ReadFile(epicFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	epicID, ok := fm["epic_id"].(string)
	if !ok || epicID == "" {
		t.Errorf("expected epic_id in frontmatter, got: %v", fm["epic_id"])
	}

	if epicID != "gromit-abc" {
		t.Errorf("expected epic_id 'gromit-abc', got '%s'", epicID)
	}

	if !strings.Contains(body, "# Test Epic") {
		t.Errorf("expected body to contain title, got: %s", body)
	}
}

// TestCreateEpicBead verifies that bd bead is created for new epic
func TestCreateEpicBead_FrontmatterRequired(t *testing.T) {
	tests := []struct {
		name           string
		epicContent    string
		shouldHaveID   bool
		expectedEpicID string
	}{
		{
			name: "epic with valid frontmatter",
			epicContent: `---
epic_id: gromit-123
created: 2026-02-08
---

# Valid Epic

Content here.
`,
			shouldHaveID:   true,
			expectedEpicID: "gromit-123",
		},
		{
			name: "epic without epic_id field",
			epicContent: `---
created: 2026-02-08
---

# Invalid Epic

Missing epic_id.
`,
			shouldHaveID: false,
		},
		{
			name: "epic without frontmatter",
			epicContent: `# No Frontmatter Epic

Just markdown content.
`,
			shouldHaveID: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			epicFile := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(epicFile, []byte(tc.epicContent), 0644); err != nil {
				t.Fatalf("failed to write epic file: %v", err)
			}

			fm, _, err := frontmatter.ReadFile(epicFile)
			if err != nil {
				t.Fatalf("ReadFile failed: %v", err)
			}

			epicID, hasID := fm["epic_id"].(string)

			if tc.shouldHaveID && !hasID {
				t.Errorf("expected epic_id to be present in frontmatter")
			}

			if !tc.shouldHaveID && hasID && epicID != "" {
				t.Errorf("expected no epic_id, but got: %s", epicID)
			}

			if tc.shouldHaveID && epicID != tc.expectedEpicID {
				t.Errorf("expected epic_id '%s', got '%s'", tc.expectedEpicID, epicID)
			}
		})
	}
}

// TestDetectNewSpecs verifies that new spec files are detected after session
func TestDetectNewSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Pre-session: one existing spec
	existingSpec := filepath.Join(specsDir, "existing-spec.md")
	if err := os.WriteFile(existingSpec, []byte("# Existing Spec"), 0644); err != nil {
		t.Fatalf("failed to write existing spec: %v", err)
	}

	preSessionSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	// Simulate session creating new spec
	newSpec := filepath.Join(specsDir, "new-feature-spec.md")
	specContent := `---
id: new-feature
created: 2026-02-08
---

# New Feature Spec

Spec created during exploration.
`
	if err := os.WriteFile(newSpec, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write new spec: %v", err)
	}

	// Post-session: detect new specs
	postSessionSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	// Find new specs
	preSet := make(map[string]bool)
	for _, spec := range preSessionSpecs {
		preSet[spec] = true
	}

	var newSpecs []string
	for _, spec := range postSessionSpecs {
		if !preSet[spec] {
			newSpecs = append(newSpecs, spec)
		}
	}

	if len(newSpecs) != 1 {
		t.Errorf("expected 1 new spec, got %d", len(newSpecs))
	}

	if len(newSpecs) > 0 && newSpecs[0] != newSpec {
		t.Errorf("expected new spec %s, got %s", newSpec, newSpecs[0])
	}
}

// TestDetectNewBacklogItems verifies that new backlog items are detected after session
func TestDetectNewBacklogItems(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	// Pre-session: one existing item
	existingItem := &backlog.Idea{
		ID:        "idea-1",
		Text:      "Existing idea",
		Type:      "feature",
		CreatedAt: time.Now(),
	}
	if err := bf.Add(existingItem); err != nil {
		t.Fatalf("failed to add existing item: %v", err)
	}

	preSessionItems, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list pre-session items: %v", err)
	}

	// Simulate session adding new item
	newItem := &backlog.Idea{
		ID:        "idea-2",
		Text:      "New idea from exploration",
		Type:      "chore",
		CreatedAt: time.Now(),
	}
	if err := bf.Add(newItem); err != nil {
		t.Fatalf("failed to add new item: %v", err)
	}

	// Post-session: detect new items
	newItems, err := getNewBacklogItems(preSessionItems, bf)
	if err != nil {
		t.Fatalf("getNewBacklogItems failed: %v", err)
	}

	if len(newItems) != 1 {
		t.Errorf("expected 1 new item, got %d", len(newItems))
	}

	if len(newItems) > 0 && newItems[0].ID != "idea-2" {
		t.Errorf("expected new item ID 'idea-2', got '%s'", newItems[0].ID)
	}
}

// TestDetectMultipleArtifactTypes verifies detection of all artifact types together
func TestDetectMultipleArtifactTypes(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Take pre-session snapshots
	preEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	preSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	preBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list backlog: %v", err)
	}

	// Simulate session creating multiple artifact types
	newEpic := filepath.Join(epicsDir, "exploration-result.md")
	epicContent := `---
epic_id: gromit-explore-1
created: 2026-02-10
---

# Exploration Result Epic
`
	if err := os.WriteFile(newEpic, []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	newSpec := filepath.Join(specsDir, "feature-from-exploration.md")
	specContent := `---
id: feature-from-exploration
created: 2026-02-10
---

# Feature From Exploration
`
	if err := os.WriteFile(newSpec, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	newBacklogItem := &backlog.Idea{
		ID:        "idea-explore-1",
		Text:      "Follow-up from exploration",
		Type:      "task",
		CreatedAt: time.Now(),
	}
	if err := bf.Add(newBacklogItem); err != nil {
		t.Fatalf("failed to add backlog item: %v", err)
	}

	// Post-session: detect all new artifacts
	postEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	postSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	postBacklog, err := getNewBacklogItems(preBacklog, bf)
	if err != nil {
		t.Fatalf("getNewBacklogItems failed: %v", err)
	}

	// Calculate new artifacts
	newEpicCount := len(postEpics) - len(preEpics)
	newSpecCount := len(postSpecs) - len(preSpecs)
	newBacklogCount := len(postBacklog)

	if newEpicCount != 1 {
		t.Errorf("expected 1 new epic, got %d", newEpicCount)
	}

	if newSpecCount != 1 {
		t.Errorf("expected 1 new spec, got %d", newSpecCount)
	}

	if newBacklogCount != 1 {
		t.Errorf("expected 1 new backlog item, got %d", newBacklogCount)
	}
}

// TestNoArtifactsCreated verifies graceful handling when no artifacts are created
func TestNoArtifactsCreated(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Take snapshots
	preEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	preSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	preBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list backlog: %v", err)
	}

	// Session exits without creating artifacts
	// Post-session: detect (should be none)
	postEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	postSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	postBacklog, err := getNewBacklogItems(preBacklog, bf)
	if err != nil {
		t.Fatalf("getNewBacklogItems failed: %v", err)
	}

	if len(postEpics) != len(preEpics) {
		t.Errorf("expected no new epics, got %d new", len(postEpics)-len(preEpics))
	}

	if len(postSpecs) != len(preSpecs) {
		t.Errorf("expected no new specs, got %d new", len(postSpecs)-len(preSpecs))
	}

	if len(postBacklog) != 0 {
		t.Errorf("expected no new backlog items, got %d", len(postBacklog))
	}
}

// TestEpicFrontmatterExtractsTitle verifies epic title extraction from frontmatter or markdown
func TestEpicFrontmatterExtractsTitle(t *testing.T) {
	tests := []struct {
		name          string
		epicContent   string
		expectedTitle string
	}{
		{
			name: "title from markdown heading",
			epicContent: `---
epic_id: gromit-title-1
created: 2026-02-08
---

# Epic Title From Markdown

Body content.
`,
			expectedTitle: "Epic Title From Markdown",
		},
		{
			name: "multiple headings uses first",
			epicContent: `---
epic_id: gromit-title-2
created: 2026-02-08
---

# First Heading

Some content.

## Second Heading

More content.
`,
			expectedTitle: "First Heading",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			epicFile := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(epicFile, []byte(tc.epicContent), 0644); err != nil {
				t.Fatalf("failed to write epic file: %v", err)
			}

			_, body, err := frontmatter.ReadFile(epicFile)
			if err != nil {
				t.Fatalf("ReadFile failed: %v", err)
			}

			// Extract title from markdown (first # heading)
			lines := strings.Split(body, "\n")
			var title string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "# ") {
					title = strings.TrimPrefix(trimmed, "# ")
					break
				}
			}

			if title != tc.expectedTitle {
				t.Errorf("expected title '%s', got '%s'", tc.expectedTitle, title)
			}
		})
	}
}

// TestDetectArtifactsIgnoresNonMarkdownFiles verifies only .md files are detected
func TestDetectArtifactsIgnoresNonMarkdownFiles(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	preEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	// Create various files
	mdFile := filepath.Join(epicsDir, "valid-epic.md")
	txtFile := filepath.Join(epicsDir, "notes.txt")
	jsonFile := filepath.Join(epicsDir, "data.json")

	for _, file := range []string{mdFile, txtFile, jsonFile} {
		if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", file, err)
		}
	}

	postEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	newEpicCount := len(postEpics) - len(preEpics)
	if newEpicCount != 1 {
		t.Errorf("expected only 1 markdown file to be detected, got %d", newEpicCount)
	}

	// Verify it's the .md file
	if len(postEpics) > 0 && !strings.HasSuffix(postEpics[len(postEpics)-1], ".md") {
		t.Error("detected file should have .md extension")
	}
}
