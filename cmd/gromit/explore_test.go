package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
)

// setupExploreTest creates a temp directory with the standard explore test
// structure: .gromit/ with templates/, CLAUDE.md, and config.
func setupExploreTest(t *testing.T) (*config.Config, string) {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project\n\nThis is project context."), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	return cfg, gromitDir
}

// --- buildExplorePrompt tests (table-driven) ---

func TestBuildExplorePrompt(t *testing.T) {
	tests := []struct {
		name            string
		setupFiles      func(t *testing.T, gromitDir string)
		args            []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "includes topic and CLAUDE.md content",
			args: []string{"Improve developer onboarding"},
			wantContains: []string{
				"Improve developer onboarding",
				"project context",
			},
		},
		{
			name: "works without topic",
			args: []string{},
			wantContains: []string{
				"# Project",
			},
		},
		{
			name: "includes RULES.md content",
			setupFiles: func(t *testing.T, gromitDir string) {
				t.Helper()
				rulesPath := filepath.Join(gromitDir, "RULES.md")
				if err := os.WriteFile(rulesPath, []byte("# Rules\n\nNever commit secrets"), 0644); err != nil {
					t.Fatalf("failed to create RULES.md: %v", err)
				}
			},
			args: []string{"test topic"},
			wantContains: []string{
				"Never commit secrets",
			},
		},
		{
			name: "includes LEARNINGS.md content",
			setupFiles: func(t *testing.T, gromitDir string) {
				t.Helper()
				learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
				content := "### 2026-02-01 | Test Learning | patterns\n\nMock implementations use function pointers."
				if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create LEARNINGS.md: %v", err)
				}
			},
			args: []string{"test topic"},
			wantContains: []string{
				"Learning",
			},
		},
		{
			name: "handles missing RULES.md and LEARNINGS.md",
			args: []string{"test topic"},
			wantContains: []string{
				"test topic",
				"# Project",
			},
		},
		{
			name: "is exploration-focused not debug-focused",
			args: []string{"Improve onboarding"},
			wantContains: []string{
				"epic",
			},
			wantNotContains: []string{
				"implement the feature",
				"write the code",
			},
		},
		{
			name: "mentions epics and specs directories",
			args: []string{"test topic"},
			wantContains: []string{
				"epic",
				"spec",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, gromitDir := setupExploreTest(t)

			if tc.setupFiles != nil {
				tc.setupFiles(t, gromitDir)
			}

			prompt, err := buildExplorePrompt(cfg, gromitDir, tc.args)
			if err != nil {
				t.Fatalf("buildExplorePrompt failed: %v", err)
			}

			if len(prompt) == 0 {
				t.Fatal("prompt should not be empty")
			}

			promptLower := strings.ToLower(prompt)
			for _, want := range tc.wantContains {
				if !strings.Contains(promptLower, strings.ToLower(want)) {
					t.Errorf("prompt should contain %q", want)
				}
			}

			for _, notWant := range tc.wantNotContains {
				if strings.Contains(promptLower, strings.ToLower(notWant)) {
					t.Errorf("prompt should not contain %q", notWant)
				}
			}
		})
	}
}

// --- getEpicFiles tests ---

func TestGetEpicFiles_RecordsExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	epic1 := filepath.Join(epicsDir, "epic-1.md")
	epic2 := filepath.Join(epicsDir, "epic-2.md")
	txtFile := filepath.Join(epicsDir, "notes.txt")

	if err := os.WriteFile(epic1, []byte("# Epic 1"), 0644); err != nil {
		t.Fatalf("failed to write epic1: %v", err)
	}
	if err := os.WriteFile(epic2, []byte("# Epic 2"), 0644); err != nil {
		t.Fatalf("failed to write epic2: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}

	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	if len(existingEpics) != 2 {
		t.Errorf("expected 2 existing epics, got %d", len(existingEpics))
	}

	epicSet := make(map[string]bool)
	for _, e := range existingEpics {
		epicSet[e] = true
	}

	if !epicSet[epic1] || !epicSet[epic2] {
		t.Error("should contain both epic files")
	}
	if epicSet[txtFile] {
		t.Error("should not include non-markdown files")
	}
}

func TestGetEpicFiles_HandlesEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles should not error on empty dir: %v", err)
	}

	if len(existingEpics) != 0 {
		t.Errorf("expected empty slice for empty dir, got %d epics", len(existingEpics))
	}
}

func TestGetEpicFiles_HandlesMissingDir(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "nonexistent")

	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles should not error on missing dir: %v", err)
	}

	if len(existingEpics) != 0 {
		t.Errorf("expected empty slice for missing dir, got %d epics", len(existingEpics))
	}
}

func TestGetEpicFiles_SnapshotOrderDoesNotMatter(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	epic1 := filepath.Join(epicsDir, "a-epic.md")
	epic2 := filepath.Join(epicsDir, "z-epic.md")
	epic3 := filepath.Join(epicsDir, "m-epic.md")

	for _, path := range []string{epic3, epic1, epic2} {
		if err := os.WriteFile(path, []byte("# Epic"), 0644); err != nil {
			t.Fatalf("failed to write epic: %v", err)
		}
	}

	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	if len(existingEpics) != 3 {
		t.Fatalf("expected 3 epics, got %d", len(existingEpics))
	}

	epicSet := make(map[string]bool)
	for _, e := range existingEpics {
		epicSet[e] = true
	}

	if !epicSet[epic1] || !epicSet[epic2] || !epicSet[epic3] {
		t.Error("all three epics should be present in snapshot regardless of order")
	}
}

// --- getSpecFiles tests ---

func TestGetSpecFiles_RecordsExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	spec1 := filepath.Join(specsDir, "spec-a.md")
	spec2 := filepath.Join(specsDir, "spec-b.md")
	if err := os.WriteFile(spec1, []byte("# Spec A"), 0644); err != nil {
		t.Fatalf("failed to write spec1: %v", err)
	}
	if err := os.WriteFile(spec2, []byte("# Spec B"), 0644); err != nil {
		t.Fatalf("failed to write spec2: %v", err)
	}

	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	if len(existingSpecs) != 2 {
		t.Errorf("expected 2 existing specs, got %d", len(existingSpecs))
	}

	specSet := make(map[string]bool)
	for _, s := range existingSpecs {
		specSet[s] = true
	}

	if !specSet[spec1] || !specSet[spec2] {
		t.Error("should contain both spec files")
	}
}

func TestGetSpecFiles_HandlesMissingDir(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "nonexistent")

	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles should not error on missing dir: %v", err)
	}

	if len(existingSpecs) != 0 {
		t.Errorf("expected empty slice for missing dir, got %d specs", len(existingSpecs))
	}
}

// --- Backlog snapshot tests ---

func TestExploreBacklog_RecordsExistingItems(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	item1 := &backlog.Idea{
		ID:        "idea-1",
		Text:      "First idea",
		Type:      "feature",
		CreatedAt: time.Now(),
	}
	item2 := &backlog.Idea{
		ID:        "idea-2",
		Text:      "Second idea",
		Type:      "bug",
		CreatedAt: time.Now(),
	}

	if err := bf.Add(item1); err != nil {
		t.Fatalf("failed to add item1: %v", err)
	}
	if err := bf.Add(item2); err != nil {
		t.Fatalf("failed to add item2: %v", err)
	}

	existingItems, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list existing items: %v", err)
	}

	if len(existingItems) != 2 {
		t.Errorf("expected 2 existing items, got %d", len(existingItems))
	}

	itemIDs := make(map[string]bool)
	for _, item := range existingItems {
		itemIDs[item.ID] = true
	}

	if !itemIDs["idea-1"] || !itemIDs["idea-2"] {
		t.Error("should contain both backlog items")
	}
}

func TestExploreBacklog_HandlesMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	existingItems, err := bf.List()
	if err != nil {
		t.Fatalf("List should not error on missing file: %v", err)
	}

	if len(existingItems) != 0 {
		t.Errorf("expected empty slice for missing file, got %d items", len(existingItems))
	}
}

// --- Snapshot integration tests ---

func TestExploreSnapshot_ContainsAllArtifacts(t *testing.T) {
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

	epicFile := filepath.Join(epicsDir, "epic-1.md")
	if err := os.WriteFile(epicFile, []byte("# Epic 1"), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	specFile := filepath.Join(specsDir, "spec-a.md")
	if err := os.WriteFile(specFile, []byte("# Spec A"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}
	if err := bf.Add(&backlog.Idea{
		ID:        "idea-1",
		Text:      "Test idea",
		Type:      "feature",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to add backlog item: %v", err)
	}

	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}
	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}
	existingBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("bf.List failed: %v", err)
	}

	if len(existingEpics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(existingEpics))
	}
	if len(existingSpecs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(existingSpecs))
	}
	if len(existingBacklog) != 1 {
		t.Errorf("expected 1 backlog item, got %d", len(existingBacklog))
	}
}

func TestExploreSnapshot_DetectsNewArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	existingEpic := filepath.Join(epicsDir, "existing.md")
	if err := os.WriteFile(existingEpic, []byte("# Existing"), 0644); err != nil {
		t.Fatalf("failed to create existing epic: %v", err)
	}

	// Take pre-session snapshot
	preSessionEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get pre-session epics: %v", err)
	}
	if len(preSessionEpics) != 1 {
		t.Fatalf("pre-session snapshot should have 1 epic, got %d", len(preSessionEpics))
	}

	// Simulate session creating a new epic
	newEpic := filepath.Join(epicsDir, "new-from-session.md")
	if err := os.WriteFile(newEpic, []byte("# New Epic"), 0644); err != nil {
		t.Fatalf("failed to create new epic: %v", err)
	}

	// Take post-session snapshot
	postSessionEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get post-session epics: %v", err)
	}
	if len(postSessionEpics) != 2 {
		t.Fatalf("post-session snapshot should have 2 epics, got %d", len(postSessionEpics))
	}

	// Detect new artifacts by comparing snapshots
	preSessionSet := make(map[string]bool)
	for _, epic := range preSessionEpics {
		preSessionSet[epic] = true
	}

	var newEpics []string
	for _, epic := range postSessionEpics {
		if !preSessionSet[epic] {
			newEpics = append(newEpics, epic)
		}
	}

	if len(newEpics) != 1 || newEpics[0] != newEpic {
		t.Errorf("should detect exactly the new epic %s, got %v", newEpic, newEpics)
	}
}
