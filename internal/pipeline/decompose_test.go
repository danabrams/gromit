package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDecompose_ValidatesDependencies verifies nil dependencies are rejected.
func TestDecompose_ValidatesDependencies(t *testing.T) {
	p := New(nil, nil)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Fatal("Decompose() with nil deps should return error")
	}

	expectedMsg := "nil dependencies"
	if err.Error() != fmt.Sprintf("pipeline: %s", expectedMsg) {
		t.Errorf("error = %q, want substring %q", err.Error(), expectedMsg)
	}
}

// TestDecompose_ChecksPlanExists verifies that non-existent plan files return an error.
func TestDecompose_ChecksPlanExists(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{}
	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   &decomposeTestBeadClient{},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "nonexistent",
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Fatal("Decompose() with nonexistent plan should return error")
	}

	// Should mention plan not found
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("error = %q, want error about plan not found", err.Error())
	}
}

// decomposeTestClaudeClient is a mock with injectable functions for decompose tests.
type decomposeTestClaudeClient struct {
	RunFn func(prompt string, model string) (interface{}, error)
}

func (m *decomposeTestClaudeClient) Run(prompt string, model string) (interface{}, error) {
	if m.RunFn != nil {
		return m.RunFn(prompt, model)
	}
	return nil, nil
}

// decomposeTestBeadClient is a mock with injectable functions for decompose tests.
type decomposeTestBeadClient struct {
	CreateWithDepsAndDescriptionFn func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error)
}

func (m *decomposeTestBeadClient) Ready() (interface{}, error) {
	return nil, nil
}

func (m *decomposeTestBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *decomposeTestBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, nil
}

func (m *decomposeTestBeadClient) Close(id string) error {
	return nil
}

func (m *decomposeTestBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	if m.CreateWithDepsAndDescriptionFn != nil {
		return m.CreateWithDepsAndDescriptionFn(title, priority, labels, criteria, deps, desc)
	}
	return nil, nil
}
