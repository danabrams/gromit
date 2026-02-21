package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClaudeClientAdapter_ConstructsTypedStruct verifies claudeClientAdapter.Run returns typed struct
func TestClaudeClientAdapter_ConstructsTypedStruct(t *testing.T) {
	// Adapters are now in adapters.go
	adapterPath := filepath.Join(".", "adapters.go")
	content, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("reading adapters.go: %v", err)
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

// TestClaudeClientAdapter_UsesConfigTimeout verifies that the adapter uses configurable timeout
// sourced from config instead of a hardcoded duration.
func TestClaudeClientAdapter_UsesConfigTimeout(t *testing.T) {
	decomposePath := filepath.Join(".", "decompose.go")
	decomposeContent, err := os.ReadFile(decomposePath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	reviewPath := filepath.Join(".", "review.go")
	reviewContent, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("reading review.go: %v", err)
	}

	decomposeStr := string(decomposeContent)
	reviewStr := string(reviewContent)

	if !adapterTestContainsString(decomposeStr, "time.Duration(cfg.Claude.Timeout) * time.Second") {
		t.Error("decompose should pass config timeout to claudeClientAdapter: Timeout: time.Duration(cfg.Claude.Timeout) * time.Second")
	}

	if !adapterTestContainsString(reviewStr, "time.Duration(cfg.Claude.Timeout) * time.Second") {
		t.Error("review should pass config timeout to claudeClientAdapter: Timeout: time.Duration(cfg.Claude.Timeout) * time.Second")
	}
}

// TestClaudeClientAdapter_NoHardcodedTimeout verifies the adapter no longer uses a fixed timeout.
func TestClaudeClientAdapter_NoHardcodedTimeout(t *testing.T) {
	adapterPath := filepath.Join(".", "adapters.go")
	content, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("reading adapters.go: %v", err)
	}

	contentStr := string(content)
	if adapterTestContainsString(contentStr, "30*time.Minute") {
		t.Error("claudeClientAdapter should not use hardcoded 30*time.Minute timeout")
	}
}

// TestBeadClientAdapter_ConstructsTypedStruct verifies beadClientAdapter methods return typed structs
func TestBeadClientAdapter_ConstructsTypedStruct(t *testing.T) {
	adapterPath := filepath.Join(".", "adapters.go")
	content, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("reading adapters.go: %v", err)
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

// TestAdapterFile_ImportsTypedPipeline verifies adapters.go imports pipeline package properly
func TestAdapterFile_ImportsTypedPipeline(t *testing.T) {
	adapterPath := filepath.Join(".", "adapters.go")
	content, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("reading adapters.go: %v", err)
	}

	contentStr := string(content)

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
	adapterPath := filepath.Join(".", "adapters.go")
	content, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("reading adapters.go: %v", err)
	}

	contentStr := string(content)

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
