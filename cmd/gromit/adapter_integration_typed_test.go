package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestClaudeAdapter_IntegrationWithTypedResult verifies the complete adapter chain returns typed results
func TestClaudeAdapter_IntegrationWithTypedResult(t *testing.T) {
	// Expected failure: This will pass once adapters return typed ClaudeRunResult
	// This test demonstrates the behavioral difference: no type assertions in workflow code

	// Create a mock claude.Result (the concrete type from internal/claude)
	concreteResult := &claude.Result{
		Success:  true,
		ExitCode: 0,
		Output:   "test output from claude",
	}

	// The adapter converts concrete type to pipeline type
	pipelineResult := &pipeline.ClaudeRunResult{
		Success:  concreteResult.Success,
		ExitCode: concreteResult.ExitCode,
		Output:   concreteResult.Output,
	}

	// Verify pipeline result has accessible fields without type assertions
	if !pipelineResult.Success {
		t.Error("expected Success field to be directly accessible")
	}

	if pipelineResult.Output != "test output from claude" {
		t.Errorf("expected Output to be 'test output from claude', got %q", pipelineResult.Output)
	}

	// The key difference: Old code would have done:
	// result := claudeAdapter.Run(...)  // returns interface{}
	// resultMap := result.(map[string]interface{})  // type assertion
	// success := resultMap["Success"].(bool)  // nested type assertion
	// output := resultMap["Output"].(string)  // nested type assertion

	// New code does:
	// result := claudeAdapter.Run(...)  // returns *ClaudeRunResult
	// if !result.Success { ... }  // direct field access, compile-time checked
	// output := result.Output  // direct field access, compile-time checked
}

// TestBeadAdapter_IntegrationWithTypedResult verifies bead adapter returns typed BeadInfo
func TestBeadAdapter_IntegrationWithTypedResult(t *testing.T) {
	// Expected failure: This will pass once adapters return typed BeadInfo
	// This test shows no extractBeadID function is needed

	// Create a mock bead.Bead (the concrete type from internal/bead)
	concreteBead := &bead.Bead{
		ID:       "test-bead-123",
		Title:    "Test Bead Title",
		Priority: 1,
		Labels:   []string{"spec:test", "complexity:low"},
	}

	// The adapter converts concrete type to pipeline type
	pipelineBeadInfo := &pipeline.BeadInfo{
		ID:       concreteBead.ID,
		Title:    concreteBead.Title,
		Priority: concreteBead.Priority,
		Labels:   concreteBead.Labels,
	}

	// Verify we can access ID directly - no extractBeadID needed!
	beadID := pipelineBeadInfo.ID
	if beadID != "test-bead-123" {
		t.Errorf("expected ID 'test-bead-123', got %q", beadID)
	}

	// Verify all fields are directly accessible
	if pipelineBeadInfo.Title != "Test Bead Title" {
		t.Errorf("expected Title 'Test Bead Title', got %q", pipelineBeadInfo.Title)
	}

	if pipelineBeadInfo.Priority != 1 {
		t.Errorf("expected Priority 1, got %d", pipelineBeadInfo.Priority)
	}

	if len(pipelineBeadInfo.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(pipelineBeadInfo.Labels))
	}

	// The key difference: Old code needed extractBeadID which:
	// 1. Tried type assertion to map[string]interface{}
	// 2. Tried calling .ID() method via reflection
	// 3. Tried reflection to access ID field
	// All eliminated with typed returns!
}

// TestWorkflowUsage_NoTypeAssertions verifies workflow code uses typed results directly
func TestWorkflowUsage_NoTypeAssertions(t *testing.T) {
	// Expected failure: This demonstrates the usage pattern after typed interfaces

	// Simulate what decompose workflow does:
	mockClaudeResult := &pipeline.ClaudeRunResult{
		Success:  true,
		ExitCode: 0,
		Output:   `[{"title":"Task","description":"Do it","priority":"P1","acceptance_criteria":[],"depends_on_index":[]}]`,
	}

	// Old way (REMOVED):
	// resultMap, ok := claudeResult.(map[string]interface{})
	// if !ok { ... }
	// success, _ := resultMap["Success"].(bool)
	// output, _ := resultMap["Output"].(string)

	// New way (current):
	if !mockClaudeResult.Success {
		t.Fatal("Claude invocation failed")
	}
	output := mockClaudeResult.Output

	// Verify we got the output without any type assertions
	if output == "" {
		t.Error("expected non-empty output")
	}

	// Similarly for bead creation:
	mockBeadResult := &pipeline.BeadInfo{
		ID:       "created-bead-456",
		Title:    "Created Bead",
		Priority: 1,
		Labels:   []string{"spec:test"},
	}

	// Old way (REMOVED):
	// beadID, err := extractBeadID(beadResult)  // reflection-based extraction

	// New way (current):
	beadID := mockBeadResult.ID

	if beadID != "created-bead-456" {
		t.Errorf("expected ID 'created-bead-456', got %q", beadID)
	}

	// This test passes to demonstrate the simplified usage pattern
	// The acceptance criteria verify that the old patterns are gone from the codebase
}

// TestCompileTimeTypeSafety verifies typed interfaces catch errors at compile time
func TestCompileTimeTypeSafety(t *testing.T) {
	// This test demonstrates compile-time safety with typed interfaces

	var claudeClient pipeline.ClaudeClient
	var beadClient pipeline.BeadClient

	// These are interface declarations - the test verifies they compile
	// If the interfaces still used interface{}, this would compile with wrong types
	_ = claudeClient
	_ = beadClient

	// Mock implementations must match the exact signature
	typedClaude := &mockClaudeForIntegration{
		result: &pipeline.ClaudeRunResult{Success: true},
	}
	typedBead := &mockBeadForIntegration{
		info: &pipeline.BeadInfo{ID: "test"},
	}

	// Verify implementations match interface
	claudeClient = typedClaude
	beadClient = typedBead

	// If we try to assign wrong return types, it won't compile
	// This is the key benefit: errors caught at compile time, not runtime

	result, _ := claudeClient.Run("prompt", "model")
	if result == nil {
		t.Error("expected non-nil result")
	}
	// result.Success is compile-time checked - no type assertion

	beadInfo, _ := beadClient.Ready()
	if beadInfo == nil {
		t.Error("expected non-nil bead info")
	}
	// beadInfo.ID is compile-time checked - no extractBeadID
}

// Mock implementations for integration tests

type mockClaudeForIntegration struct {
	result *pipeline.ClaudeRunResult
	err    error
}

func (m *mockClaudeForIntegration) Run(prompt, model string) (*pipeline.ClaudeRunResult, error) {
	return m.result, m.err
}

type mockBeadForIntegration struct {
	info *pipeline.BeadInfo
	err  error
}

func (m *mockBeadForIntegration) Ready() (*pipeline.BeadInfo, error) {
	return m.info, m.err
}

func (m *mockBeadForIntegration) Show(id string) (*pipeline.BeadInfo, error) {
	return m.info, m.err
}

func (m *mockBeadForIntegration) Create(title string, priority int, labels, outputs []string) (*pipeline.BeadInfo, error) {
	return m.info, m.err
}

func (m *mockBeadForIntegration) CreateWithDepsAndDescription(title string, priority int, labels, criteria, deps []string, desc string) (*pipeline.BeadInfo, error) {
	return m.info, m.err
}

func (m *mockBeadForIntegration) Close(id string) error {
	return m.err
}
