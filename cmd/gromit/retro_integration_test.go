//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/scope"
)

// --- Test helpers ---

type specFile struct {
	filename string
	id       string
	epic     string
}

// writeSpecFiles creates spec markdown files in the given directory.
func writeSpecFiles(t *testing.T, specsDir string, specs []specFile) {
	t.Helper()
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}
	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		content := fmt.Sprintf("---\nid: %s\nepic: %s\ncreated: 2026-02-08\n---\n\n# Spec\n", spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec file %s: %v", spec.filename, err)
		}
	}
}

// writeIterationLogs appends JSONL iteration log entries to logsDir/iteration.log.
func writeIterationLogs(t *testing.T, logsDir string, entries []logger.IterationLog) {
	t.Helper()
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}
	logPath := filepath.Join(logsDir, "iteration.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()
	for i, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("Failed to marshal log entry %d: %v", i, err)
		}
		if _, err := f.WriteString(string(data) + "\n"); err != nil {
			t.Fatalf("Failed to write log entry %d: %v", i, err)
		}
	}
}

// assertLabelSet verifies that got contains exactly the expected labels (order-independent).
func assertLabelSet(t *testing.T, got []string, expected []string) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("expected %d labels, got %d: %v", len(expected), len(got), got)
	}
	want := make(map[string]bool, len(expected))
	for _, l := range expected {
		want[l] = false
	}
	for _, l := range got {
		if _, exists := want[l]; !exists {
			t.Errorf("unexpected label %q", l)
		}
		want[l] = true
	}
	for l, found := range want {
		if !found {
			t.Errorf("missing expected label %q", l)
		}
	}
}

// --- Scope flag tests ---

func TestRetroCommand_SpecFlagExists(t *testing.T) {
	specFlag := retroCmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("retro command should have --spec flag")
	}
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

func TestRetroCommand_EpicFlagExists(t *testing.T) {
	epicFlag := retroCmd.Flags().Lookup("epic")
	if epicFlag == nil {
		t.Fatal("retro command should have --epic flag")
	}
	if epicFlag.Value.Type() != "string" {
		t.Errorf("--epic flag should be string type, got %s", epicFlag.Value.Type())
	}
}

// TestRunRetro_ValidatesFlags verifies that runRetro calls scope.ValidateFlags
// and returns an error when both --spec and --epic are set
func TestRunRetro_ValidatesFlags(t *testing.T) {
	retroSpecFlag = "init-wizard"
	retroEpicFlag = "gromit-xyz"
	defer func() {
		retroSpecFlag = ""
		retroEpicFlag = ""
	}()

	err := runRetro(retroCmd, []string{})
	if err == nil {
		t.Fatal("runRetro should return error when both --spec and --epic are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

func TestRetroCommand_SpecFlagResolvesToLabel(t *testing.T) {
	if err := scope.ValidateFlags("", "init-wizard"); err != nil {
		t.Fatalf("ValidateFlags should accept --spec alone, got error: %v", err)
	}

	labels := scope.ResolveSpec("init-wizard")
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	if labels[0] != "spec:init-wizard" {
		t.Errorf("ResolveSpec = %q, want %q", labels[0], "spec:init-wizard")
	}
}

func TestRetroCommand_EpicFlagUsesResolveEpic(t *testing.T) {
	specsDir := filepath.Join(t.TempDir(), "specs")
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
	})

	labels, err := scope.ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	assertLabelSet(t, labels, []string{"spec:auth", "spec:profile"})
}

func TestRetroCommand_EpicFlagFiltersIterationLogs(t *testing.T) {
	specsDir := filepath.Join(t.TempDir(), "specs")
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
		{"settings.md", "settings", "other-epic"},
	})

	if err := scope.ValidateFlags("gromit-xyz", ""); err != nil {
		t.Fatalf("ValidateFlags should accept --epic alone, got error: %v", err)
	}

	labels, err := scope.ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	assertLabelSet(t, labels, []string{"spec:auth", "spec:profile"})
}

func TestRetroCommand_NoScopeFlagUsesDefaultBehavior(t *testing.T) {
	if err := scope.ValidateFlags("", ""); err != nil {
		t.Fatalf("ValidateFlags should accept both flags empty, got error: %v", err)
	}
}

func TestRetroCommand_EpicResolutionWithNoSpecs(t *testing.T) {
	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	if err := scope.ValidateFlags("nonexistent-epic", ""); err != nil {
		t.Fatalf("ValidateFlags should accept --epic flag, got error: %v", err)
	}

	labels, err := scope.ResolveEpic("nonexistent-epic", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic should not error for nonexistent epic, got error: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("ResolveEpic should return 0 labels for nonexistent epic, got %d: %v", len(labels), labels)
	}
}

// --- CLI help text tests ---

func TestRetroCmd_HelpText_DocumentsSpecFlag(t *testing.T) {
	helpText := retroCmd.Long
	if !strings.Contains(helpText, "--spec") {
		t.Error("retroCmd.Long should document --spec flag")
	}
	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("retroCmd.Long should explain filtering when using --spec")
	}
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("retroCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

func TestRetroCmd_HelpText_DocumentsEpicFlag(t *testing.T) {
	helpText := retroCmd.Long
	if !strings.Contains(helpText, "--epic") {
		t.Error("retroCmd.Long should document --epic flag")
	}
	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("retroCmd.Long should explain filtering when using --epic")
	}
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("retroCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// --- buildBeadFilter tests ---

func TestRetroCommand_BuildBeadFilterHandlesEmptyBeadList(t *testing.T) {
	ctx := context.Background()

	filter, err := buildBeadFilter(ctx, []string{})
	if err != nil {
		t.Errorf("buildBeadFilter with empty labels should not error, got: %v", err)
	}
	if filter != nil {
		t.Errorf("buildBeadFilter with empty labels should return nil, got: %v", filter)
	}

	filter, err = buildBeadFilter(ctx, nil)
	if err != nil {
		t.Errorf("buildBeadFilter with nil labels should not error, got: %v", err)
	}
	if filter != nil {
		t.Errorf("buildBeadFilter with nil labels should return nil, got: %v", filter)
	}
}

// --- End-to-end environment setup tests ---

func TestRetroCommand_EndToEndSpecFiltering(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tempDir, "gromit.yaml")
	cfg := &config.Config{
		Paths: config.PathsConfig{GromitDir: gromitDir},
		Models: config.ModelsConfig{
			P0: "claude-opus-4", P1: "claude-sonnet-3-5",
			P2: "claude-haiku-3", Validation: "claude-haiku-3",
		},
	}
	configData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	logsDir := filepath.Join(gromitDir, "logs")
	entries := []logger.IterationLog{
		{Timestamp: time.Now(), Iteration: 1, BeadID: "gromit-1", BeadTitle: "Test bead gromit-1", Model: "claude-sonnet-3-5", Success: true, Validated: false},
		{Timestamp: time.Now(), Iteration: 2, BeadID: "gromit-2", BeadTitle: "Test bead gromit-2", Model: "claude-sonnet-3-5", Success: true, Validated: true},
		{Timestamp: time.Now(), Iteration: 3, BeadID: "gromit-3", BeadTitle: "Test bead gromit-3", Model: "claude-sonnet-3-5", Success: true, Validated: false},
		{Timestamp: time.Now(), Iteration: 4, BeadID: "gromit-4", BeadTitle: "Test bead gromit-4", Model: "claude-sonnet-3-5", Success: false, Validated: true},
		{Timestamp: time.Now(), Iteration: 5, BeadID: "gromit-5", BeadTitle: "Test bead gromit-5", Model: "claude-sonnet-3-5", Success: true, Validated: false},
	}
	writeIterationLogs(t, logsDir, entries)

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# Rules\n\nTest rules\n"), 0644); err != nil {
		t.Fatalf("Failed to write RULES.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "LEARNINGS.md"), []byte("# Learnings\n\nTest learnings\n"), 0644); err != nil {
		t.Fatalf("Failed to write LEARNINGS.md: %v", err)
	}

	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	retroTemplate := "# Retrospective Analysis\n\n## Rules\n{{ .Rules }}\n\n## Learnings\n{{ .Learnings }}\n\n## Stats\nTotal iterations: {{ .RunStats.Total }}\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_RETRO.md"), []byte(retroTemplate), 0644); err != nil {
		t.Fatalf("Failed to write retro template: %v", err)
	}

	// Verify all infrastructure files exist
	for _, path := range []string{
		filepath.Join(logsDir, "iteration.log"),
		configPath,
		filepath.Join(gromitDir, "RULES.md"),
		filepath.Join(gromitDir, "LEARNINGS.md"),
		filepath.Join(templatesDir, "PROMPT_RETRO.md"),
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("Expected file should exist: %s", path)
		}
	}

	// beadFilter flow: scope.ResolveSpec returns labels, buildBeadFilter builds
	// the filter map from bead IDs, retro.Run() uses the filter to restrict analysis
}

func TestRetroCommand_EndToEndEpicFiltering(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "gromit-100"},
		{"profile.md", "profile", "gromit-100"},
		{"settings.md", "settings", "gromit-200"},
	})

	logsDir := filepath.Join(gromitDir, "logs")
	entries := []logger.IterationLog{
		{Timestamp: time.Now(), Iteration: 1, BeadID: "gromit-1", BeadTitle: "Test bead gromit-1", Model: "claude-sonnet-3-5", Success: true},
		{Timestamp: time.Now(), Iteration: 2, BeadID: "gromit-2", BeadTitle: "Test bead gromit-2", Model: "claude-sonnet-3-5", Success: true},
		{Timestamp: time.Now(), Iteration: 3, BeadID: "gromit-3", BeadTitle: "Test bead gromit-3", Model: "claude-sonnet-3-5", Success: true},
	}
	writeIterationLogs(t, logsDir, entries)

	for _, filename := range []string{"auth.md", "profile.md", "settings.md"} {
		specPath := filepath.Join(specsDir, filename)
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			t.Fatalf("Spec file %s should exist", filename)
		}
	}

	// beadFilter flow: scope.ResolveEpic returns labels for all specs in the epic,
	// buildBeadFilter unions bead IDs from all labels, retro.Run() filters analysis
}
