package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClaudeClientAdapter_ConstructsTypedStruct verifies claudeClientAdapter.Run returns typed struct
// Expected failure: claudeClientAdapter.Run() currently constructs map[string]interface{} at lines 233-237
func TestClaudeClientAdapter_ConstructsTypedStruct(t *testing.T) {
	// Verify adapter constructs pipeline.ClaudeRunResult, not map[string]interface{}
	decomposeAdapterPath := filepath.Join(".", "decompose.go")
	content, err := os.ReadFile(decomposeAdapterPath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Check that claudeClientAdapter.Run constructs &pipeline.ClaudeRunResult{...}
	if !adapterTestContainsString(contentStr, "&pipeline.ClaudeRunResult{") {
		t.Error("claudeClientAdapter.Run() should construct &pipeline.ClaudeRunResult{...}, not map[string]interface{}")
	}

	// Check that map[string]interface{} construction is removed
	if adapterTestContainsString(contentStr, `map[string]interface{}{`) && adapterTestContainsInClaudeAdapter(contentStr) {
		t.Error("claudeClientAdapter.Run() still constructs map[string]interface{}, should use typed struct")
	}
}

// TestBeadClientAdapter_ConstructsTypedStruct verifies beadClientAdapter methods return typed structs
// Expected failure: beadClientAdapter methods currently return interface{} wrapping *bead.Bead directly
func TestBeadClientAdapter_ConstructsTypedStruct(t *testing.T) {
	decomposeAdapterPath := filepath.Join(".", "decompose.go")
	content, err := os.ReadFile(decomposeAdapterPath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

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

// TestAdapterFile_ImportsTypedPipeline verifies decompose.go imports pipeline package properly
// Expected failure: After implementation, decompose.go should use pipeline.ClaudeRunResult and pipeline.BeadInfo
func TestAdapterFile_ImportsTypedPipeline(t *testing.T) {
	decomposeAdapterPath := filepath.Join(".", "decompose.go")
	content, err := os.ReadFile(decomposeAdapterPath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Verify file imports pipeline package
	if !adapterTestContainsImport(contentStr, "github.com/danabrams/gromit/internal/pipeline") {
		t.Error("decompose.go should import github.com/danabrams/gromit/internal/pipeline")
	}

	// Verify usage of typed pipeline types (not interface{})
	if !adapterTestContainsString(contentStr, "pipeline.ClaudeRunResult") {
		t.Error("decompose.go should reference pipeline.ClaudeRunResult type")
	}

	if !adapterTestContainsString(contentStr, "pipeline.BeadInfo") {
		t.Error("decompose.go should reference pipeline.BeadInfo type")
	}
}

// TestAdapterSimplification_NoMapConstruction verifies adapters don't construct intermediate maps
// Expected failure: Current adapters construct map[string]interface{} as an intermediate representation
func TestAdapterSimplification_NoMapConstruction(t *testing.T) {
	decomposeAdapterPath := filepath.Join(".", "decompose.go")
	content, err := os.ReadFile(decomposeAdapterPath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Extract claude adapter section (between claudeClientAdapter type and beadClientAdapter type)
	claudeAdapterSection := adapterTestExtractBetween(contentStr,
		"type claudeClientAdapter struct",
		"type beadClientAdapter struct")

	if claudeAdapterSection != "" && adapterTestContainsString(claudeAdapterSection, "map[string]interface{}") {
		t.Error("claudeClientAdapter should construct typed structs directly, not intermediate maps")
	}

	// Extract bead adapter section (between beadClientAdapter type and next section)
	beadAdapterSection := adapterTestExtractBetween(contentStr,
		"type beadClientAdapter struct",
		"func convertToBeadDefs")

	if beadAdapterSection != "" && adapterTestContainsString(beadAdapterSection, "return a.Client") {
		// If it's just returning a.Client directly without constructing pipeline.BeadInfo,
		// that's the old behavior
		if !adapterTestContainsString(beadAdapterSection, "&pipeline.BeadInfo{") {
			t.Error("beadClientAdapter should construct &pipeline.BeadInfo{...}, not return bead.Bead directly")
		}
	}
}

// Helper functions for string analysis

func adapterTestContainsString(content, substr string) bool {
	return len(content) > 0 && len(substr) > 0 && adapterTestIndexString(content, substr) >= 0
}

func adapterTestIndexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
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

	endIdx := adapterTestIndexString(content[startIdx:], end)
	if endIdx < 0 {
		return content[startIdx:]
	}

	return content[startIdx : startIdx+endIdx]
}
