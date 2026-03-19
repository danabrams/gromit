package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_DoneMalformedDateExcludedFromPickerAndShownWithoutDate(t *testing.T) {
	// Seed: create specs directory with two spec files
	specsDir := t.TempDir()

	// 0003b.md starts with "DONE not-a-date"
	err := os.WriteFile(
		filepath.Join(specsDir, "0003b.md"),
		[]byte("DONE not-a-date\n# Spec 0003b — Replan Context Deduplication\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("write 0003b.md: %v", err)
	}

	// 0003c.md — no DONE prefix, should be eligible
	err = os.WriteFile(
		filepath.Join(specsDir, "0003c.md"),
		[]byte("# Spec 0003c — Review Stage Graceful Degradation\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("write 0003c.md: %v", err)
	}

	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// --- Part 1: Picker excludes DONE spec with malformed date ---

	var pickerOut bytes.Buffer
	pickerIn := strings.NewReader("1\n")
	branchResolver := func(path string) string { return "main" }

	result, err := pickSpec("test-project", specsDir, store, branchResolver, pickerIn, &pickerOut)
	if err != nil {
		t.Fatalf("pickSpec: %v", err)
	}

	pickerOutput := pickerOut.String()

	// Assert: 0003b (DONE with malformed date) should NOT appear in picker
	if strings.Contains(pickerOutput, "0003b") {
		t.Errorf("picker should not show done spec 0003b with malformed date, got:\n%s", pickerOutput)
	}

	// Assert: 0003c should appear in picker
	if !strings.Contains(pickerOutput, "0003c") {
		t.Errorf("picker should show 0003c, got:\n%s", pickerOutput)
	}

	// Assert: selecting option 1 should return 0003c
	expectedPath := filepath.Join(specsDir, "0003c.md")
	if result != expectedPath {
		t.Errorf("expected selected path %q, got %q", expectedPath, result)
	}

	// --- Part 2: spec list shows 0003b as "done" with no date in parentheses ---

	cmd := newSpecListCmd()
	var listBuf bytes.Buffer
	cmd.SetOut(&listBuf)
	cmd.SetArgs([]string{
		"--project", "test-project",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spec list cmd.Execute: %v", err)
	}
	listOutput := listBuf.String()

	// Assert: 0003b shows "done" status
	lines := strings.Split(listOutput, "\n")
	found0003b := false
	for _, line := range lines {
		if strings.Contains(line, "0003b") {
			found0003b = true
			if !strings.Contains(line, "done") {
				t.Errorf("expected 0003b line to contain 'done', got: %s", line)
			}
			// Assert: no date in parentheses (malformed date should not be shown)
			if strings.Contains(line, "(not-a-date)") {
				t.Errorf("expected no malformed date in parentheses for 0003b, got: %s", line)
			}
			if strings.Contains(line, "(") {
				t.Errorf("expected no parenthesized date for 0003b with malformed date, got: %s", line)
			}
		}
	}
	if !found0003b {
		t.Errorf("expected 0003b in spec list output:\n%s", listOutput)
	}
}