package specloop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

func TestScenario_PromotedValidationGapAppearsInValidatorPrompt(t *testing.T) {
	// === Seed ===
	// Create a playbook with an active validation_gap entry
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbook: %v", err)
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	entries := []playbook.Entry{
		{
			ID:               "pb-gap-fragile",
			Type:             "validation_gap",
			Title:            "File-path-specific contract assertions are fragile",
			Content:          "Assertions that check for exact file paths break when files are moved or renamed. Prefer checking for behavioral outcomes instead of path-specific patterns.",
			Rationale:        "Multiple runs have failed due to path-based assertions that became stale after refactoring.",
			Status:           "active",
			SourceProposalID: "prop-gap-001",
			SourceRunID:      "run-gap-001",
			SourceSpecID:     "spec-gap-001",
			CreatedAt:        time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
			SupersededBy:     "",
		},
	}
	if err := pbStore.Save(entries); err != nil {
		t.Fatalf("save playbook entries: %v", err)
	}

	workDir := t.TempDir()
	runDir := t.TempDir()

	// === Invoke ===
	// Create a FileTaskContextProvider that loads from our seeded cellPath
	ctxProvider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)

	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return workDir })
	runner.SetContextProvider(ctxProvider)

	// Fix task — validation gaps are included for fix/repair tasks (includeSpec=true)
	task := runstore.Task{
		TaskID:    "t-fix-gap-check",
		Objective: "fix failing contract assertions after file rename",
		Kind:      "fix",
		FailuresAddressed: []string{
			"contract:scenario-contracts.yaml — file_contains failed: pattern not found in renamed file",
		},
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	prompt := inv.capturedPrompt

	// === Assert ===
	// The Known Validation Gaps section must appear in the prompt
	if !strings.Contains(prompt, "### Known Validation Gaps") {
		t.Error("prompt should contain 'Known Validation Gaps' section header")
	}

	// The gap title must appear so the validator knows about the known issue
	if !strings.Contains(prompt, "File-path-specific contract assertions are fragile") {
		t.Error("prompt should contain the validation gap title")
	}

	// The gap content must appear so the validator understands the issue details
	if !strings.Contains(prompt, "Assertions that check for exact file paths break when files are moved or renamed") {
		t.Error("prompt should contain the validation gap content")
	}

	// The rationale must appear so the validator understands why this gap matters
	if !strings.Contains(prompt, "Multiple runs have failed due to path-based assertions") {
		t.Error("prompt should contain the validation gap rationale")
	}
}

func TestScenario_PromotedValidationGapAppearsInRepairPrompt(t *testing.T) {
	// === Seed ===
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbook: %v", err)
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	entries := []playbook.Entry{
		{
			ID:        "pb-gap-fragile",
			Type:      "validation_gap",
			Title:     "File-path-specific contract assertions are fragile",
			Content:   "Assertions that check for exact file paths break when files are moved or renamed. Prefer checking for behavioral outcomes instead of path-specific patterns.",
			Rationale: "Multiple runs have failed due to path-based assertions that became stale after refactoring.",
			Status:    "active",
			CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
	}
	if err := pbStore.Save(entries); err != nil {
		t.Fatalf("save playbook entries: %v", err)
	}

	workDir := t.TempDir()
	runDir := t.TempDir()

	// === Invoke ===
	ctxProvider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)

	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return workDir })
	runner.SetContextProvider(ctxProvider)

	// RepairTask always uses includeSpec=true, so validation gaps are included
	task := runstore.Task{
		TaskID:    "t-repair-gap-check",
		Objective: "repair contract assertion that broke after file move",
	}
	failures := []string{
		"contract:scenario-contracts.yaml — file_contains failed: expected pattern in old path",
	}

	_, err := runner.RepairTask(context.Background(), task, failures)
	if err != nil {
		t.Fatalf("RepairTask: %v", err)
	}

	prompt := inv.capturedPrompt

	// === Assert ===
	if !strings.Contains(prompt, "### Known Validation Gaps") {
		t.Error("repair prompt should contain 'Known Validation Gaps' section header")
	}
	if !strings.Contains(prompt, "File-path-specific contract assertions are fragile") {
		t.Error("repair prompt should contain the validation gap title")
	}
	if !strings.Contains(prompt, "Assertions that check for exact file paths break when files are moved or renamed") {
		t.Error("repair prompt should contain the validation gap content")
	}
}

func TestScenario_ValidationGapOmittedFromOriginalTaskPrompt(t *testing.T) {
	// === Seed ===
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("mkdir playbook: %v", err)
	}

	pbStore := &playbook.Store{Dir: playbookDir}
	entries := []playbook.Entry{
		{
			ID:        "pb-gap-fragile",
			Type:      "validation_gap",
			Title:     "File-path-specific contract assertions are fragile",
			Content:   "Assertions that check for exact file paths break when files are moved or renamed.",
			Status:    "active",
			CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
	}
	if err := pbStore.Save(entries); err != nil {
		t.Fatalf("save playbook entries: %v", err)
	}

	workDir := t.TempDir()
	runDir := t.TempDir()

	// === Invoke ===
	ctxProvider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)

	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return workDir })
	runner.SetContextProvider(ctxProvider)

	// Original task (not fix) — validation gaps should NOT be included
	task := runstore.Task{
		TaskID:    "t-original-no-gaps",
		Objective: "implement new feature",
		Kind:      "",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	prompt := inv.capturedPrompt

	// === Assert ===
	// Original tasks should NOT include validation gaps (only fix/repair tasks do)
	if strings.Contains(prompt, "### Known Validation Gaps") {
		t.Error("original task prompt should NOT contain 'Known Validation Gaps' section")
	}
	if strings.Contains(prompt, "File-path-specific contract assertions are fragile") {
		t.Error("original task prompt should NOT contain validation gap content")
	}
}
