//go:build acceptance

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/frontmatter"
)

// TestExplorePostSessionArtifactDetection verifies the full post-session
// artifact detection flow for gromit explore.
//
// This is an acceptance test that requires:
// - bd CLI to be installed and in PATH
// - Ability to create bd beads (may require bd workspace initialization)
//
// The test simulates the explore session workflow:
// 1. Take pre-session snapshots of epics, specs, and backlog
// 2. Simulate Claude session creating artifacts
// 3. Detect new artifacts by comparing snapshots
// 4. Create bd beads for new epics with type=epic
// 5. Report new specs and backlog items
func TestExplorePostSessionArtifactDetection(t *testing.T) {
	// Check if bd is available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd CLI not found in PATH, skipping acceptance test")
	}

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	// Create directory structure
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Initialize bd workspace in temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Try to initialize bd workspace
	bdInitCmd := exec.Command("bd", "init")
	if err := bdInitCmd.Run(); err != nil {
		t.Skipf("failed to initialize bd workspace: %v", err)
	}

	// Create some existing artifacts to ensure snapshots work correctly
	existingEpic := filepath.Join(epicsDir, "existing-epic.md")
	existingEpicContent := `---
epic_id: gromit-existing
created: 2026-02-01
---

# Existing Epic

This epic existed before the session.
`
	if err := os.WriteFile(existingEpic, []byte(existingEpicContent), 0644); err != nil {
		t.Fatalf("failed to write existing epic: %v", err)
	}

	// Take pre-session snapshots
	preEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get pre-session epics: %v", err)
	}
	if len(preEpics) != 1 {
		t.Fatalf("expected 1 existing epic, got %d", len(preEpics))
	}

	preSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("failed to get pre-session specs: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}
	preBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list pre-session backlog: %v", err)
	}

	// Simulate Claude session creating new artifacts
	t.Run("detect and create bead for new epic", func(t *testing.T) {
		// Create a new epic file (simulating Claude's output)
		newEpicPath := filepath.Join(epicsDir, "onboarding-improvements.md")
		newEpicContent := `---
epic_id: gromit-onboarding-123
created: 2026-02-10
---

# Improve Developer Onboarding

## Problem

New developers struggle to get started with the project quickly.

## Exploration

After researching various approaches, we identified three key areas:
1. Documentation clarity
2. Setup automation
3. Interactive tutorials

## Next Steps

Create specs for each area.
`
		if err := os.WriteFile(newEpicPath, []byte(newEpicContent), 0644); err != nil {
			t.Fatalf("failed to create new epic: %v", err)
		}

		// Post-session: detect new epics
		postEpics, err := getEpicFiles(epicsDir)
		if err != nil {
			t.Fatalf("failed to get post-session epics: %v", err)
		}

		// Find new epics
		preSet := make(map[string]bool)
		for _, epic := range preEpics {
			preSet[epic] = true
		}

		var newEpics []string
		for _, epic := range postEpics {
			if !preSet[epic] {
				newEpics = append(newEpics, epic)
			}
		}

		if len(newEpics) != 1 {
			t.Fatalf("expected 1 new epic, got %d", len(newEpics))
		}

		// Read frontmatter to get epic_id
		fm, body, err := frontmatter.ReadFile(newEpics[0])
		if err != nil {
			t.Fatalf("failed to read epic frontmatter: %v", err)
		}

		epicID, ok := fm["epic_id"].(string)
		if !ok || epicID == "" {
			t.Fatalf("epic frontmatter missing epic_id field")
		}

		if epicID != "gromit-onboarding-123" {
			t.Errorf("expected epic_id 'gromit-onboarding-123', got '%s'", epicID)
		}

		// Extract title from markdown body
		title := extractTitleFromMarkdown(body)
		if title == "" {
			title = "Unnamed Epic"
		}

		// Create bd bead with type=epic
		createCmd := exec.Command("bd", "create", title, "--type", "epic")
		output, err := createCmd.CombinedOutput()
		if err != nil {
			t.Logf("bd create output: %s", string(output))
			t.Fatalf("failed to create bd bead for epic: %v", err)
		}

		// Verify the bead was created by checking bd list
		listCmd := exec.Command("bd", "list", "--type", "epic", "--json")
		listOutput, err := listCmd.Output()
		if err != nil {
			t.Fatalf("failed to list bd beads: %v", err)
		}

		if !strings.Contains(string(listOutput), title) {
			t.Errorf("bd bead for epic '%s' not found in bd list output", title)
		}

		t.Logf("✓ Created bd bead for epic: %s", title)
	})

	t.Run("detect and report new specs", func(t *testing.T) {
		// Create a new spec file
		newSpecPath := filepath.Join(specsDir, "setup-automation.md")
		newSpecContent := `---
id: setup-automation
epic: gromit-onboarding-123
created: 2026-02-10
---

# Setup Automation

Automate the project setup process for new developers.
`
		if err := os.WriteFile(newSpecPath, []byte(newSpecContent), 0644); err != nil {
			t.Fatalf("failed to create new spec: %v", err)
		}

		// Detect new specs
		postSpecs, err := getSpecFiles(specsDir)
		if err != nil {
			t.Fatalf("failed to get post-session specs: %v", err)
		}

		preSpecSet := make(map[string]bool)
		for _, spec := range preSpecs {
			preSpecSet[spec] = true
		}

		var newSpecs []string
		for _, spec := range postSpecs {
			if !preSpecSet[spec] {
				newSpecs = append(newSpecs, spec)
			}
		}

		if len(newSpecs) != 1 {
			t.Fatalf("expected 1 new spec, got %d", len(newSpecs))
		}

		// Verify the new spec has the expected content
		fm, _, err := frontmatter.ReadFile(newSpecs[0])
		if err != nil {
			t.Fatalf("failed to read spec frontmatter: %v", err)
		}

		specID, ok := fm["id"].(string)
		if !ok || specID != "setup-automation" {
			t.Errorf("expected spec id 'setup-automation', got '%v'", fm["id"])
		}

		epicRef, ok := fm["epic"].(string)
		if !ok || epicRef != "gromit-onboarding-123" {
			t.Errorf("expected spec epic 'gromit-onboarding-123', got '%v'", fm["epic"])
		}

		t.Logf("✓ Detected new spec: %s (linked to epic: %s)", specID, epicRef)
	})

	t.Run("detect and report new backlog items", func(t *testing.T) {
		// Add a new backlog item
		newItem := &backlog.Idea{
			ID:        backlog.GenerateID(),
			Text:      "Research interactive tutorial frameworks",
			Type:      "research",
			CreatedAt: time.Now(),
		}
		if err := bf.Add(newItem); err != nil {
			t.Fatalf("failed to add backlog item: %v", err)
		}

		// Detect new backlog items
		newBacklogItems, err := getNewBacklogItems(preBacklog, bf)
		if err != nil {
			t.Fatalf("failed to get new backlog items: %v", err)
		}

		if len(newBacklogItems) != 1 {
			t.Fatalf("expected 1 new backlog item, got %d", len(newBacklogItems))
		}

		if newBacklogItems[0].Text != "Research interactive tutorial frameworks" {
			t.Errorf("expected specific backlog text, got: %s", newBacklogItems[0].Text)
		}

		t.Logf("✓ Detected new backlog item: %s", newBacklogItems[0].Text)
	})
}

// TestExploreNoArtifactsCreated verifies graceful handling when exploration
// session exits without creating any artifacts.
func TestExploreNoArtifactsCreated(t *testing.T) {
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
		t.Fatalf("failed to get pre-session epics: %v", err)
	}

	preSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("failed to get pre-session specs: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}
	preBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list pre-session backlog: %v", err)
	}

	// Session exits without creating anything
	// (user decided not to pursue the exploration)

	// Post-session detection should find no new artifacts
	postEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get post-session epics: %v", err)
	}

	postSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("failed to get post-session specs: %v", err)
	}

	newBacklog, err := getNewBacklogItems(preBacklog, bf)
	if err != nil {
		t.Fatalf("failed to get new backlog items: %v", err)
	}

	// Verify no new artifacts detected
	if len(postEpics) != len(preEpics) {
		t.Errorf("expected no new epics, but found %d new", len(postEpics)-len(preEpics))
	}

	if len(postSpecs) != len(preSpecs) {
		t.Errorf("expected no new specs, but found %d new", len(postSpecs)-len(preSpecs))
	}

	if len(newBacklog) != 0 {
		t.Errorf("expected no new backlog items, but found %d", len(newBacklog))
	}

	t.Log("✓ Correctly handled exploration session with no artifacts created")
}

// TestExploreMultipleEpicsCreated verifies handling when multiple epics
// are created in a single session.
func TestExploreMultipleEpicsCreated(t *testing.T) {
	// Check if bd is available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd CLI not found in PATH, skipping acceptance test")
	}

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Initialize bd workspace
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	bdInitCmd := exec.Command("bd", "init")
	if err := bdInitCmd.Run(); err != nil {
		t.Skipf("failed to initialize bd workspace: %v", err)
	}

	// Take pre-session snapshot
	preEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get pre-session epics: %v", err)
	}

	// Simulate session creating multiple epics
	epicsData := []struct {
		filename string
		epicID   string
		title    string
	}{
		{
			filename: "api-improvements.md",
			epicID:   "gromit-api-1",
			title:    "API Improvements",
		},
		{
			filename: "ui-modernization.md",
			epicID:   "gromit-ui-1",
			title:    "UI Modernization",
		},
	}

	for _, data := range epicsData {
		epicPath := filepath.Join(epicsDir, data.filename)
		epicContent := fmt.Sprintf(`---
epic_id: %s
created: 2026-02-10
---

# %s

Epic content here.
`, data.epicID, data.title)

		if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
			t.Fatalf("failed to create epic %s: %v", data.filename, err)
		}
	}

	// Detect new epics
	postEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get post-session epics: %v", err)
	}

	preSet := make(map[string]bool)
	for _, epic := range preEpics {
		preSet[epic] = true
	}

	var newEpics []string
	for _, epic := range postEpics {
		if !preSet[epic] {
			newEpics = append(newEpics, epic)
		}
	}

	if len(newEpics) != 2 {
		t.Fatalf("expected 2 new epics, got %d", len(newEpics))
	}

	// Create bd beads for each new epic
	createdCount := 0
	for _, epicPath := range newEpics {
		fm, body, err := frontmatter.ReadFile(epicPath)
		if err != nil {
			t.Fatalf("failed to read epic frontmatter: %v", err)
		}

		epicID, ok := fm["epic_id"].(string)
		if !ok || epicID == "" {
			t.Errorf("epic %s missing epic_id in frontmatter", epicPath)
			continue
		}

		title := extractTitleFromMarkdown(body)
		if title == "" {
			title = "Unnamed Epic"
		}

		// Create bd bead
		createCmd := exec.Command("bd", "create", title, "--type", "epic")
		if output, err := createCmd.CombinedOutput(); err != nil {
			t.Logf("bd create output: %s", string(output))
			t.Errorf("failed to create bd bead for epic %s: %v", title, err)
			continue
		}

		createdCount++
		t.Logf("✓ Created bd bead for epic: %s (id: %s)", title, epicID)
	}

	if createdCount != 2 {
		t.Errorf("expected to create 2 bd beads, created %d", createdCount)
	}
}

// extractTitleFromMarkdown extracts the first H1 heading from markdown content
func extractTitleFromMarkdown(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}
	return ""
}
