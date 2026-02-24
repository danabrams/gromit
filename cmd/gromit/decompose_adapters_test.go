package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
)

// TestClaudeClientAdapter_ConstructsTypedStruct verifies claudeClientAdapter.Run returns typed struct
func TestClaudeClientAdapter_ConstructsTypedStruct(t *testing.T) {
	contentStr := adapterTestReadFile(t, "adapters.go")

	// Check that claudeClientAdapter.Run constructs &pipeline.ClaudeRunResult{...}
	if !adapterTestContainsString(contentStr, "&pipeline.ClaudeRunResult{") {
		t.Error("claudeClientAdapter.Run() should construct &pipeline.ClaudeRunResult{...}, not map[string]interface{}")
	}

	// Check that map[string]interface{} construction is removed
	if adapterTestContainsString(contentStr, `map[string]interface{}{`) && adapterTestContainsInClaudeAdapter(contentStr) {
		t.Error("claudeClientAdapter.Run() still constructs map[string]interface{}, should use typed struct")
	}
}

// TestClaudeClientAdapter_UsesConfigTimeout verifies that the adapter uses configurable timeout
// sourced from config instead of a hardcoded duration.
func TestClaudeClientAdapter_UsesConfigTimeout(t *testing.T) {
	slowScript := adapterTestCreateSleepScript(t, 2*time.Second, "done")
	adapter := adapterTestNewClaudeAdapter(t, slowScript, 50*time.Millisecond)

	start := time.Now()
	_, err := adapter.Run("prompt", "haiku")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed >= time.Second {
		t.Fatalf("adapter did not honor configured timeout; elapsed=%v", elapsed)
	}
}

// TestClaudeClientAdapter_NoHardcodedTimeout verifies the adapter no longer uses a fixed timeout.
func TestClaudeClientAdapter_NoHardcodedTimeout(t *testing.T) {
	slowScript := adapterTestCreateSleepScript(t, 200*time.Millisecond, "ok")
	shortTimeoutAdapter := adapterTestNewClaudeAdapter(t, slowScript, 50*time.Millisecond)
	longTimeoutAdapter := adapterTestNewClaudeAdapter(t, slowScript, 2*time.Second)

	if _, err := shortTimeoutAdapter.Run("prompt", "haiku"); err == nil {
		t.Fatal("expected short timeout adapter to fail")
	}

	result, err := longTimeoutAdapter.Run("prompt", "haiku")
	if err != nil {
		t.Fatalf("long timeout adapter unexpectedly failed: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("long timeout adapter result = %#v, want success", result)
	}
}

// TestBeadClientAdapter_ConstructsTypedStruct verifies beadClientAdapter methods return typed structs
func TestBeadClientAdapter_ConstructsTypedStruct(t *testing.T) {
	contentStr := adapterTestReadFile(t, "adapters.go")

	// Check that adapter constructs &pipeline.BeadInfo{...}
	if !adapterTestContainsString(contentStr, "&pipeline.BeadInfo{") {
		t.Error("beadClientAdapter methods should construct &pipeline.BeadInfo{...}")
	}

	// Verify beadClientAdapter has expected methods returning typed values
	requiredMethods := []string{
		"func (a *beadClientAdapter) Ready() (*pipeline.BeadInfo, error)",
		"func (a *beadClientAdapter) Show(id string) (*pipeline.BeadInfo, error)",
		"func (a *beadClientAdapter) Create(",
		"func (a *beadClientAdapter) CreateWithDepsAndDescription(",
	}

	for _, method := range requiredMethods {
		if !adapterTestContainsString(contentStr, method) {
			t.Errorf("beadClientAdapter missing expected method signature: %s", method)
		}
	}
}

// TestAdapterFile_ImportsTypedPipeline verifies adapters.go imports pipeline package properly
func TestAdapterFile_ImportsTypedPipeline(t *testing.T) {
	contentStr := adapterTestReadFile(t, "adapters.go")

	// Verify file imports pipeline package
	if !adapterTestContainsImport(contentStr, "github.com/danabrams/gromit/internal/pipeline") {
		t.Error("adapters.go should import github.com/danabrams/gromit/internal/pipeline")
	}

	// Verify usage of typed pipeline types (not interface{})
	if !adapterTestContainsString(contentStr, "pipeline.ClaudeRunResult") {
		t.Error("adapters.go should reference pipeline.ClaudeRunResult type")
	}

	if !adapterTestContainsString(contentStr, "pipeline.BeadInfo") {
		t.Error("adapters.go should reference pipeline.BeadInfo type")
	}
}

// TestAdapterSimplification_NoMapConstruction verifies adapters don't construct intermediate maps
func TestAdapterSimplification_NoMapConstruction(t *testing.T) {
	contentStr := adapterTestReadFile(t, "adapters.go")

	// Extract claude adapter section (between claudeClientAdapter type and beadClientAdapter type)
	claudeAdapterSection := adapterTestExtractBetween(contentStr,
		"type claudeClientAdapter struct",
		"type beadClientAdapter struct")

	if claudeAdapterSection != "" && adapterTestContainsString(claudeAdapterSection, "map[string]interface{}") {
		t.Error("claudeClientAdapter should construct typed structs directly, not intermediate maps")
	}

	// Extract bead adapter section (between beadClientAdapter type and end of file)
	beadAdapterSection := adapterTestExtractBetween(contentStr,
		"type beadClientAdapter struct",
		"")

	if beadAdapterSection != "" && adapterTestContainsString(beadAdapterSection, "return a.Client") {
		// If it's just returning a.Client directly without constructing pipeline.BeadInfo,
		// that's the old behavior
		if !adapterTestContainsString(beadAdapterSection, "&pipeline.BeadInfo{") {
			t.Error("beadClientAdapter should construct &pipeline.BeadInfo{...}, not return bead.Bead directly")
		}
	}
}

func TestAdapterHelpers_UseLLMRunResultNaming(t *testing.T) {
	contentStr := adapterTestReadFile(t, "adapters.go")

	if !adapterTestContainsString(contentStr, "func toLLMRunResult(") {
		t.Fatal("adapters.go should define helper func toLLMRunResult")
	}
	if adapterTestContainsString(contentStr, "func toClaudeRunResult(") {
		t.Fatal("adapters.go should not define helper func toClaudeRunResult")
	}
}

// Helper functions for string analysis

func adapterTestContainsString(content, substr string) bool {
	return strings.Contains(content, substr)
}

func adapterTestIndexString(s, substr string) int {
	return strings.Index(s, substr)
}

func adapterTestContainsImport(content, pkg string) bool {
	// Check for both single-line and multi-line import styles
	singleLine := `import "` + pkg + `"`
	multiLineQuoted := `"` + pkg + `"`

	return adapterTestContainsString(content, singleLine) || adapterTestContainsString(content, multiLineQuoted)
}

func adapterTestContainsInClaudeAdapter(content string) bool {
	// Check if map[string]interface{} appears in claudeClientAdapter section
	adapterSection := adapterTestExtractBetween(content,
		"func (a *claudeClientAdapter) Run",
		"type beadClientAdapter")
	return adapterTestContainsString(adapterSection, "map[string]interface{}")
}

func adapterTestExtractBetween(content, start, end string) string {
	startIdx := adapterTestIndexString(content, start)
	if startIdx < 0 {
		return ""
	}

	if end == "" {
		return content[startIdx:]
	}

	endIdx := adapterTestIndexString(content[startIdx:], end)
	if endIdx < 0 {
		return content[startIdx:]
	}

	return content[startIdx : startIdx+endIdx]
}

func adapterTestReadFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(".", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return string(content)
}

func adapterTestCreateSleepScript(t *testing.T, sleep time.Duration, output string) string {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "mock-claude.sh")
	content := fmt.Sprintf("#!/bin/sh\nsleep %.3f\necho %q\n", sleep.Seconds(), output)
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatalf("writing mock claude script: %v", err)
	}
	return scriptPath
}

func adapterTestNewClaudeAdapter(t *testing.T, scriptPath string, timeout time.Duration) *claudeClientAdapter {
	t.Helper()

	client, err := claude.NewClient(scriptPath, nil, 30)
	if err != nil {
		t.Fatalf("creating claude client: %v", err)
	}

	return &claudeClientAdapter{
		Client:  client,
		Timeout: timeout,
	}
}
