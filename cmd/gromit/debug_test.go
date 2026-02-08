package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
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

func TestContainsFile(t *testing.T) {
	slice := []string{"/path/to/file1.md", "/path/to/file2.md"}

	if !containsFile(slice, "/path/to/file1.md") {
		t.Error("expected containsFile to return true for file1.md")
	}

	if containsFile(slice, "/path/to/file3.md") {
		t.Error("expected containsFile to return false for file3.md")
	}

	if containsFile([]string{}, "/path/to/file1.md") {
		t.Error("expected containsFile to return false for empty slice")
	}
}
