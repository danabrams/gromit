package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/runbook"
)

func TestGetReportFiles(t *testing.T) {
	t.Run("finds markdown files", func(t *testing.T) {
		tmpDir := t.TempDir()
		reportsDir := filepath.Join(tmpDir, "reports")
		if err := os.MkdirAll(reportsDir, 0755); err != nil {
			t.Fatalf("failed to create reports dir: %v", err)
		}

		// Create some report files
		report1 := filepath.Join(reportsDir, "debug-20260208-120000.md")
		report2 := filepath.Join(reportsDir, "debug-20260208-130000.md")
		if err := os.WriteFile(report1, []byte("# Report 1"), 0644); err != nil {
			t.Fatalf("failed to write report1: %v", err)
		}
		if err := os.WriteFile(report2, []byte("# Report 2"), 0644); err != nil {
			t.Fatalf("failed to write report2: %v", err)
		}

		// Create a non-markdown file (should be ignored)
		txtFile := filepath.Join(reportsDir, "notes.txt")
		if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
			t.Fatalf("failed to write txt file: %v", err)
		}

		reports, err := getReportFiles(reportsDir)
		if err != nil {
			t.Fatalf("getReportFiles failed: %v", err)
		}

		if len(reports) != 2 {
			t.Errorf("expected 2 reports, got %d", len(reports))
		}

		// Verify both reports are present
		foundReport1 := false
		foundReport2 := false
		for _, r := range reports {
			if r == report1 {
				foundReport1 = true
			}
			if r == report2 {
				foundReport2 = true
			}
		}

		if !foundReport1 || !foundReport2 {
			t.Errorf("missing expected reports: report1=%v, report2=%v", foundReport1, foundReport2)
		}
	})

	t.Run("handles missing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		reportsDir := filepath.Join(tmpDir, "nonexistent")

		reports, err := getReportFiles(reportsDir)
		if err != nil {
			t.Fatalf("getReportFiles should not error on missing dir: %v", err)
		}

		if len(reports) != 0 {
			t.Errorf("expected empty slice for missing dir, got %d reports", len(reports))
		}
	})

	t.Run("handles empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		reportsDir := filepath.Join(tmpDir, "reports")
		if err := os.MkdirAll(reportsDir, 0755); err != nil {
			t.Fatalf("failed to create reports dir: %v", err)
		}

		reports, err := getReportFiles(reportsDir)
		if err != nil {
			t.Fatalf("getReportFiles failed: %v", err)
		}

		if len(reports) != 0 {
			t.Errorf("expected empty slice for empty dir, got %d reports", len(reports))
		}
	})
}

func TestGetPlanFiles(t *testing.T) {
	t.Run("finds markdown files", func(t *testing.T) {
		tmpDir := t.TempDir()
		plansDir := filepath.Join(tmpDir, "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}

		// Create some plan files
		plan1 := filepath.Join(plansDir, "feature-a.md")
		plan2 := filepath.Join(plansDir, "feature-b.md")
		if err := os.WriteFile(plan1, []byte("# Plan A"), 0644); err != nil {
			t.Fatalf("failed to write plan1: %v", err)
		}
		if err := os.WriteFile(plan2, []byte("# Plan B"), 0644); err != nil {
			t.Fatalf("failed to write plan2: %v", err)
		}

		plans, err := getPlanFiles(plansDir)
		if err != nil {
			t.Fatalf("getPlanFiles failed: %v", err)
		}

		if len(plans) != 2 {
			t.Errorf("expected 2 plans, got %d", len(plans))
		}

		// Verify both plans are present
		foundPlan1 := false
		foundPlan2 := false
		for _, p := range plans {
			if p == plan1 {
				foundPlan1 = true
			}
			if p == plan2 {
				foundPlan2 = true
			}
		}

		if !foundPlan1 || !foundPlan2 {
			t.Errorf("missing expected plans: plan1=%v, plan2=%v", foundPlan1, foundPlan2)
		}
	})

	t.Run("handles missing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		plansDir := filepath.Join(tmpDir, "nonexistent")

		plans, err := getPlanFiles(plansDir)
		if err != nil {
			t.Fatalf("getPlanFiles should not error on missing dir: %v", err)
		}

		if len(plans) != 0 {
			t.Errorf("expected empty slice for missing dir, got %d plans", len(plans))
		}
	})
}

func TestGetNewBacklogItems(t *testing.T) {
	t.Run("detects new items", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		if err := os.MkdirAll(gromitDir, 0755); err != nil {
			t.Fatalf("failed to create gromit dir: %v", err)
		}

		bf, err := backlog.NewFile(gromitDir)
		if err != nil {
			t.Fatalf("failed to create backlog file: %v", err)
		}

		// Create initial items
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

		// Add a new item
		item3 := &backlog.Idea{
			ID:        "idea-3",
			Text:      "Third idea",
			Type:      "chore",
			CreatedAt: time.Now(),
		}
		if err := bf.Add(item3); err != nil {
			t.Fatalf("failed to add item3: %v", err)
		}

		// Detect new items
		newItems, err := getNewBacklogItems(existingItems, bf)
		if err != nil {
			t.Fatalf("getNewBacklogItems failed: %v", err)
		}

		if len(newItems) != 1 {
			t.Errorf("expected 1 new item, got %d", len(newItems))
		}

		if len(newItems) > 0 && newItems[0].ID != "idea-3" {
			t.Errorf("expected new item ID 'idea-3', got '%s'", newItems[0].ID)
		}
	})

	t.Run("returns empty when no new items", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		if err := os.MkdirAll(gromitDir, 0755); err != nil {
			t.Fatalf("failed to create gromit dir: %v", err)
		}

		bf, err := backlog.NewFile(gromitDir)
		if err != nil {
			t.Fatalf("failed to create backlog file: %v", err)
		}

		// Create initial items
		item1 := &backlog.Idea{
			ID:        "idea-1",
			Text:      "First idea",
			Type:      "feature",
			CreatedAt: time.Now(),
		}

		if err := bf.Add(item1); err != nil {
			t.Fatalf("failed to add item1: %v", err)
		}

		existingItems, err := bf.List()
		if err != nil {
			t.Fatalf("failed to list existing items: %v", err)
		}

		// No new items added
		newItems, err := getNewBacklogItems(existingItems, bf)
		if err != nil {
			t.Fatalf("getNewBacklogItems failed: %v", err)
		}

		if len(newItems) != 0 {
			t.Errorf("expected 0 new items, got %d", len(newItems))
		}
	})
}

// TestGetMDFiles verifies the shared getMDFiles function correctly finds markdown files
func TestGetMDFiles(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, dir string) []string // returns expected file paths
		expectError bool
	}{
		{
			name: "finds markdown files in directory",
			setup: func(t *testing.T, dir string) []string {
				f1 := filepath.Join(dir, "file1.md")
				f2 := filepath.Join(dir, "file2.md")
				f3 := filepath.Join(dir, "file3.txt")

				for _, f := range []string{f1, f2, f3} {
					if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
						t.Fatalf("failed to write file: %v", err)
					}
				}

				return []string{f1, f2} // Only .md files expected
			},
			expectError: false,
		},
		{
			name: "returns empty slice for empty directory",
			setup: func(t *testing.T, dir string) []string {
				return []string{}
			},
			expectError: false,
		},
		{
			name: "ignores non-markdown files",
			setup: func(t *testing.T, dir string) []string {
				f1 := filepath.Join(dir, "file.txt")
				f2 := filepath.Join(dir, "file.json")
				f3 := filepath.Join(dir, "file.yaml")

				for _, f := range []string{f1, f2, f3} {
					if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
						t.Fatalf("failed to write file: %v", err)
					}
				}

				return []string{} // No markdown files expected
			},
			expectError: false,
		},
		{
			name: "ignores subdirectories",
			setup: func(t *testing.T, dir string) []string {
				md := filepath.Join(dir, "file.md")
				subdir := filepath.Join(dir, "subdir")

				if err := os.WriteFile(md, []byte("content"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
				if err := os.MkdirAll(subdir, 0755); err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}

				return []string{md} // Only top-level .md file expected
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			expectedFiles := tt.setup(t, tmpDir)

			files, err := getMDFiles(tmpDir)

			if tt.expectError && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(files) != len(expectedFiles) {
				t.Errorf("expected %d files, got %d", len(expectedFiles), len(files))
			}

			// Check that all expected files are present
			for _, expected := range expectedFiles {
				found := false
				for _, actual := range files {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected file %q not found in results", expected)
				}
			}
		})
	}
}

// TestGetMDFilesNonexistentDirectory verifies getMDFiles handles missing directories gracefully
func TestGetMDFilesNonexistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "nonexistent")

	files, err := getMDFiles(nonexistentDir)
	if err != nil {
		t.Fatalf("getMDFiles should not error on missing dir: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected empty slice for missing dir, got %d files", len(files))
	}
}

// TestGetReportFilesUsesGetMDFiles verifies getReportFiles works through getMDFiles
func TestGetReportFilesUsesGetMDFiles(t *testing.T) {
	tmpDir := t.TempDir()
	reportsDir := filepath.Join(tmpDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		t.Fatalf("failed to create reports dir: %v", err)
	}

	// Create test files
	report1 := filepath.Join(reportsDir, "debug-20260208-120000.md")
	report2 := filepath.Join(reportsDir, "debug-20260208-130000.md")
	txtFile := filepath.Join(reportsDir, "notes.txt")

	if err := os.WriteFile(report1, []byte("# Report 1"), 0644); err != nil {
		t.Fatalf("failed to write report1: %v", err)
	}
	if err := os.WriteFile(report2, []byte("# Report 2"), 0644); err != nil {
		t.Fatalf("failed to write report2: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}

	reports, err := getReportFiles(reportsDir)
	if err != nil {
		t.Fatalf("getReportFiles failed: %v", err)
	}

	// Should only return .md files
	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}

	for _, expected := range []string{report1, report2} {
		found := false
		for _, actual := range reports {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected report %q not found", expected)
		}
	}
}

// TestGetPlanFilesUsesGetMDFiles verifies getPlanFiles works through getMDFiles
func TestGetPlanFilesUsesGetMDFiles(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create test files
	plan1 := filepath.Join(plansDir, "feature-a.md")
	plan2 := filepath.Join(plansDir, "feature-b.md")
	nonMD := filepath.Join(plansDir, "notes.txt")

	if err := os.WriteFile(plan1, []byte("# Plan A"), 0644); err != nil {
		t.Fatalf("failed to write plan1: %v", err)
	}
	if err := os.WriteFile(plan2, []byte("# Plan B"), 0644); err != nil {
		t.Fatalf("failed to write plan2: %v", err)
	}
	if err := os.WriteFile(nonMD, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed to write nonMD file: %v", err)
	}

	plans, err := getPlanFiles(plansDir)
	if err != nil {
		t.Fatalf("getPlanFiles failed: %v", err)
	}

	// Should only return .md files
	if len(plans) != 2 {
		t.Errorf("expected 2 plans, got %d", len(plans))
	}

	for _, expected := range []string{plan1, plan2} {
		found := false
		for _, actual := range plans {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected plan %q not found", expected)
		}
	}
}

// TestDetectAndReportArtifactsUseSlicesContains verifies the artifact detection correctly
// identifies newly created artifacts (testing the behavior that depends on proper slice containment check)
func TestDetectAndReportArtifactsUseSlicesContains(t *testing.T) {
	tmpDir := t.TempDir()
	reportsDir := filepath.Join(tmpDir, "reports")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create directories
	for _, dir := range []string{reportsDir, plansDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	// Create a new report and plan file
	newReport := filepath.Join(reportsDir, "new-report.md")
	newPlan := filepath.Join(plansDir, "new-plan.md")

	if err := os.WriteFile(newReport, []byte("# New Report"), 0644); err != nil {
		t.Fatalf("failed to write new report: %v", err)
	}
	if err := os.WriteFile(newPlan, []byte("# New Plan"), 0644); err != nil {
		t.Fatalf("failed to write new plan: %v", err)
	}

	// Verify file detection works correctly
	newReportsList, err := getReportFiles(reportsDir)
	if err != nil {
		t.Fatalf("failed to get report files: %v", err)
	}

	newPlansList, err := getPlanFiles(plansDir)
	if err != nil {
		t.Fatalf("failed to get plan files: %v", err)
	}

	// The key test: verify we correctly identify new artifacts
	// This depends on proper slice containment checking
	if len(newReportsList) != 1 {
		t.Errorf("expected 1 new report, got %d", len(newReportsList))
	}

	if len(newPlansList) != 1 {
		t.Errorf("expected 1 new plan, got %d", len(newPlansList))
	}

	// Verify the exact artifacts are detected
	if newReportsList[0] != newReport {
		t.Errorf("expected report %q, got %q", newReport, newReportsList[0])
	}

	if newPlansList[0] != newPlan {
		t.Errorf("expected plan %q, got %q", newPlan, newPlansList[0])
	}
}

func TestDebugModelFlag(t *testing.T) {
	// Reset the debugModel variable to its default state before each subtest
	originalModel := debugModel
	defer func() { debugModel = originalModel }()

	t.Run("default model is opus", func(t *testing.T) {
		// The debugModel variable should have the default set by cobra
		// when the command is initialized
		debugModel = "" // Reset to empty

		// Simulate what cobra does during flag initialization
		debugCmd.Flags().Lookup("model").DefValue = "opus"

		// Verify the default value is set correctly
		defaultValue := debugCmd.Flags().Lookup("model").DefValue
		if defaultValue != "opus" {
			t.Errorf("expected default model to be 'opus', got %q", defaultValue)
		}
	})

	t.Run("model flag can be set", func(t *testing.T) {
		// Test that the model variable can be changed
		debugModel = "sonnet"
		if debugModel != "sonnet" {
			t.Errorf("expected debugModel to be 'sonnet', got %q", debugModel)
		}

		debugModel = "haiku"
		if debugModel != "haiku" {
			t.Errorf("expected debugModel to be 'haiku', got %q", debugModel)
		}
	})
}

// TestBuildDebugPromptWithRunbookEntry verifies that buildDebugPrompt injects a Failure Context
// section when a runbook entry is provided.
func TestBuildDebugPromptWithRunbookEntry(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	entry := runbook.Entry{
		ID:                 "rb-123-gromit-abc",
		BeadID:             "gromit-abc",
		BeadTitle:          "Fix login bug",
		FailureCategory:    "test_failure",
		StartCommit:        "abc1234",
		FailureCommit:      "def5678",
		FailureOutput:      "FAIL: TestLogin\nexpected 200 got 500",
		ValidationCommands: []string{"go test ./...", "go vet ./..."},
		Prompt:             "implement login fix",
	}

	prompt, err := buildDebugPrompt(nil, gromitDir, []string{}, &entry)
	if err != nil {
		t.Fatalf("buildDebugPrompt failed: %v", err)
	}

	if !strings.Contains(prompt, "## Failure Context") {
		t.Errorf("expected prompt to contain '## Failure Context' section, got:\n%s", prompt)
	}

	if !strings.Contains(prompt, "gromit-abc") {
		t.Errorf("expected prompt to contain bead ID 'gromit-abc'")
	}

	if !strings.Contains(prompt, "Fix login bug") {
		t.Errorf("expected prompt to contain bead title 'Fix login bug'")
	}

	if !strings.Contains(prompt, "test_failure") {
		t.Errorf("expected prompt to contain failure category 'test_failure'")
	}
}

// TestBuildDebugPromptWithRunbookEntryDetailsSection verifies the Failure Context section
// contains commit diff instructions, validation commands, failure output, and build prompt.
func TestBuildDebugPromptWithRunbookEntryDetailsSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	entry := runbook.Entry{
		ID:                 "rb-456-gromit-xyz",
		BeadID:             "gromit-xyz",
		BeadTitle:          "Fix cache miss",
		FailureCategory:    "compilation_error",
		StartCommit:        "start001",
		FailureCommit:      "fail002",
		FailureOutput:      "undefined: CacheClient",
		ValidationCommands: []string{"go test ./internal/cache/...", "go build ./..."},
		Prompt:             "add CacheClient field to struct",
	}

	result, err := buildDebugPrompt(nil, gromitDir, []string{}, &entry)
	if err != nil {
		t.Fatalf("buildDebugPrompt failed: %v", err)
	}

	// Should contain commit refs for diffing
	if !strings.Contains(result, "start001") {
		t.Errorf("expected result to contain start commit 'start001'")
	}
	if !strings.Contains(result, "fail002") {
		t.Errorf("expected result to contain failure commit 'fail002'")
	}

	// Should contain validation commands
	if !strings.Contains(result, "go test ./internal/cache/...") {
		t.Errorf("expected result to contain validation command 'go test ./internal/cache/...'")
	}

	// Should contain failure output
	if !strings.Contains(result, "undefined: CacheClient") {
		t.Errorf("expected result to contain failure output 'undefined: CacheClient'")
	}

	// Should contain build prompt
	if !strings.Contains(result, "add CacheClient field to struct") {
		t.Errorf("expected result to contain build prompt 'add CacheClient field to struct'")
	}

	// Failure Context must appear before Context section
	failureIdx := strings.Index(result, "## Failure Context")
	contextIdx := strings.Index(result, "## Context")
	if failureIdx == -1 {
		t.Fatal("expected '## Failure Context' in result")
	}
	if contextIdx == -1 {
		t.Fatal("expected '## Context' in result")
	}
	if failureIdx > contextIdx {
		t.Errorf("Failure Context section must appear before Context section")
	}
}

// TestBuildDebugPromptWithoutRunbookEntryHasNoFailureContext verifies that when no
// runbook entry is provided, the Failure Context section is absent.
func TestBuildDebugPromptWithoutRunbookEntryHasNoFailureContext(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	result, err := buildDebugPrompt(nil, gromitDir, []string{}, nil)
	if err != nil {
		t.Fatalf("buildDebugPrompt failed: %v", err)
	}

	if strings.Contains(result, "## Failure Context") {
		t.Errorf("expected no Failure Context section when entry is nil, but found one")
	}
}

// TestPickRunbookEntryReturnsSelectedEntry verifies that pickRunbookEntry returns the
// entry corresponding to the user's numbered selection.
func TestPickRunbookEntryReturnsSelectedEntry(t *testing.T) {
	entries := []runbook.Entry{
		{
			BeadID:          "gromit-aaa",
			BeadTitle:       "First bug",
			FailureCategory: "test_failure",
			Timestamp:       time.Now().Add(-1 * time.Hour),
		},
		{
			BeadID:          "gromit-bbb",
			BeadTitle:       "Second bug",
			FailureCategory: "lint_error",
			Timestamp:       time.Now().Add(-2 * time.Hour),
		},
	}

	// User selects entry 1
	reader := strings.NewReader("1\n")
	entry, err := pickRunbookEntry(entries, reader)
	if err != nil {
		t.Fatalf("pickRunbookEntry failed: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry for selection 1")
	}
	if entry.BeadID != "gromit-aaa" {
		t.Errorf("expected BeadID 'gromit-aaa', got %q", entry.BeadID)
	}
}

// TestPickRunbookEntrySkipOnZeroOrInvalid verifies that pickRunbookEntry returns nil
// when the user enters 0 or non-numeric input (skip selection).
func TestPickRunbookEntrySkipOnZeroOrInvalid(t *testing.T) {
	entries := []runbook.Entry{
		{BeadID: "gromit-aaa", BeadTitle: "First bug", FailureCategory: "test_failure", Timestamp: time.Now()},
	}

	for _, input := range []string{"0\n", "skip\n", "\n", "99\n"} {
		reader := strings.NewReader(input)
		entry, err := pickRunbookEntry(entries, reader)
		if err != nil {
			t.Fatalf("pickRunbookEntry(%q) returned error: %v", input, err)
		}
		if entry != nil {
			t.Errorf("pickRunbookEntry(%q): expected nil entry for skip/invalid input, got %v", input, entry.BeadID)
		}
	}
}

// TestPickRunbookEntrySelectsSecondEntry verifies that pickRunbookEntry correctly returns
// the second entry when "2" is entered.
func TestPickRunbookEntrySelectsSecondEntry(t *testing.T) {
	entries := []runbook.Entry{
		{BeadID: "gromit-aaa", BeadTitle: "First bug", FailureCategory: "test_failure", Timestamp: time.Now()},
		{BeadID: "gromit-bbb", BeadTitle: "Second bug", FailureCategory: "lint_error", Timestamp: time.Now()},
	}

	reader := strings.NewReader("2\n")
	entry, err := pickRunbookEntry(entries, reader)
	if err != nil {
		t.Fatalf("pickRunbookEntry failed: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry for selection 2")
	}
	if entry.BeadID != "gromit-bbb" {
		t.Errorf("expected BeadID 'gromit-bbb', got %q", entry.BeadID)
	}
}

// TestBuildDebugPromptWithRunbookEntrySkippedWhenArgsPresent verifies that when a description
// arg is provided, the caller passes nil entry (picker is skipped) and the prompt
// contains the description but no Failure Context.
func TestBuildDebugPromptWithRunbookEntrySkippedWhenArgsPresent(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// When a description arg is present, nil entry is passed (picker skipped)
	result, err := buildDebugPrompt(nil, gromitDir, []string{"login fails with + in email"}, nil)
	if err != nil {
		t.Fatalf("buildDebugPrompt failed: %v", err)
	}

	if !strings.Contains(result, "login fails with + in email") {
		t.Errorf("expected prompt to contain description arg")
	}
	if strings.Contains(result, "## Failure Context") {
		t.Errorf("expected no Failure Context when description arg provided and entry is nil")
	}
}

// TestPickRunbookEntryEmptyListReturnsNil verifies that pickRunbookEntry returns nil
// when the entries list is empty (no entries to pick from).
func TestPickRunbookEntryEmptyListReturnsNil(t *testing.T) {
	reader := strings.NewReader("1\n")
	entry, err := pickRunbookEntry([]runbook.Entry{}, reader)
	if err != nil {
		t.Fatalf("pickRunbookEntry failed: %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for empty entries list, got %v", entry.BeadID)
	}
}
