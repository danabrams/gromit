package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/validate"
)

// TestDecomposeWorkflow_E2E verifies the complete Decompose workflow through Pipeline.Decompose()
// Expected failure: Pipeline.Decompose() implementation is incomplete and does not orchestrate the full workflow
func TestDecomposeWorkflow_E2E(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a plan file with frontmatter
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "authentication.md")
	planContent := `---
spec: authentication
created: 2026-02-11
---

# Authentication Plan

## Phase 1: Database Schema
- Create users table
- Add session storage

## Phase 2: API Endpoints
- POST /login endpoint
- POST /refresh endpoint
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock Claude client that returns bead definitions
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			// Verify prompt contains plan body (not frontmatter)
			if !strings.Contains(prompt, "Database Schema") {
				return nil, fmt.Errorf("prompt missing plan body content")
			}
			if strings.Contains(prompt, "spec: authentication") {
				return nil, fmt.Errorf("prompt should not contain frontmatter")
			}

			// Return JSON array of bead definitions
			jsonOutput := `[
				{
					"title": "Create database schema for users",
					"description": "Implement users table with columns for email, password hash, session tokens",
					"priority": "P1",
					"acceptance_criteria": [
						"Users table created with proper schema",
						"Migration script runs without errors"
					],
					"depends_on_index": []
				},
				{
					"title": "Implement login endpoint",
					"description": "Add POST /login handler with email/password validation",
					"priority": "P1",
					"acceptance_criteria": [
						"Endpoint returns 200 on valid credentials",
						"Endpoint returns 401 on invalid credentials",
						"Session token generated on success"
					],
					"depends_on_index": [0]
				}
			]`
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   jsonOutput,
			}, nil
		},
	}

	var createdBeads []*BeadInfo
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			bead := &BeadInfo{
				ID:       fmt.Sprintf("bead-%d", len(createdBeads)+1),
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}
			createdBeads = append(createdBeads, bead)
			return bead, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		GromitDir: tmpDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	// Execute Decompose workflow
	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "authentication",
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Decompose() returned nil result")
	}

	// Verify beads were created
	if len(result.CreatedBeads) != 2 {
		t.Errorf("CreatedBeads count = %d, want 2", len(result.CreatedBeads))
	}
	if result.PromptDiagnostics == nil {
		t.Fatal("PromptDiagnostics = nil, want populated diagnostics")
	}
	if result.PromptDiagnostics.PromptType != decomposePromptType {
		t.Errorf("PromptDiagnostics.PromptType = %q, want %q", result.PromptDiagnostics.PromptType, decomposePromptType)
	}
	if result.PromptDiagnostics.EstimatedTokens == 0 {
		t.Error("PromptDiagnostics.EstimatedTokens = 0, want non-zero estimate")
	}

	// Verify plan frontmatter was updated
	if !result.PlanUpdated {
		t.Error("PlanUpdated = false, want true")
	}

	// Read plan file and verify frontmatter was actually updated
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading plan file: %v", err)
	}
	planStr := string(planData)
	if !strings.Contains(planStr, "decomposed: true") {
		t.Error("Plan frontmatter missing 'decomposed: true'")
	}
	if !strings.Contains(planStr, "decomposed_at:") {
		t.Error("Plan frontmatter missing 'decomposed_at' timestamp")
	}

	// Verify ValidationStats for clean first attempt (no retries)
	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if result.ValidationStats.Attempts != 1 {
		t.Errorf("ValidationStats.Attempts = %d, want 1", result.ValidationStats.Attempts)
	}
	if result.ValidationStats.ViolationCount != 0 {
		t.Errorf("ValidationStats.ViolationCount = %d, want 0", result.ValidationStats.ViolationCount)
	}
	if result.ValidationStats.Improved {
		t.Error("ValidationStats.Improved = true, want false (no retries needed)")
	}
}

func TestDecomposeWorkflow_ProviderReturnsEmptyOutput(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "empty-output.md")
	if err := os.WriteFile(planPath, []byte("# Empty Output Plan\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   "   \n\t",
			}, nil
		},
	}

	p := New(&Deps{
		ClaudeClient: mockClaude,
	}, &Paths{
		GromitDir: tmpDir,
		PlansDir:  plansDir,
	})

	_, err := p.Decompose(context.Background(), DecomposeInput{PlanName: "empty-output"})
	if err == nil {
		t.Fatal("Decompose() with empty provider output returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "provider returned empty output for decompose") {
		t.Fatalf("Decompose() error = %v, want message about empty provider output", err)
	}
}

func TestDecomposeWorkflow_RetriesOnValidationViolation(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "retry-plan.md")
	if err := os.WriteFile(planPath, []byte("# Retry Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			if runCount == 2 && !strings.Contains(prompt, "Violations By Flagged Bead") {
				t.Fatalf("second prompt should be validation reprompt, got: %q", prompt)
			}

			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{
							"title": "Bad task",
							"description": "Too many criteria",
							"priority": "P1",
							"acceptance_criteria": ["a", "b", "c", "d"],
							"depends_on_index": []
						},
						{
							"title": "Good task",
							"description": "Valid from start",
							"priority": "P1",
							"acceptance_criteria": ["x", "y"],
							"depends_on_index": []
						}
					]`,
				}, nil
			}

			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Fixed task",
						"description": "Valid",
						"priority": "P1",
						"acceptance_criteria": ["a", "b", "c"],
						"depends_on_index": []
					},
					{
						"title": "Good task",
						"description": "Valid from start",
						"priority": "P1",
						"acceptance_criteria": ["x", "y"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "retry-plan",
		MaxValidationRetries: 2,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}
	if runCount != 2 {
		t.Fatalf("Run() call count = %d, want 2", runCount)
	}

	// Verify ValidationStats
	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if result.ValidationStats.Attempts != 2 {
		t.Errorf("ValidationStats.Attempts = %d, want 2", result.ValidationStats.Attempts)
	}
	if result.ValidationStats.ViolationCount != 1 {
		t.Errorf("ValidationStats.ViolationCount = %d, want 1", result.ValidationStats.ViolationCount)
	}
	if !result.ValidationStats.Improved {
		t.Error("ValidationStats.Improved = false, want true (retry fixed violations)")
	}
}

func TestDecomposeWorkflow_ValidationRetriesExhaustedContinues(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "retry-exhausted.md")
	if err := os.WriteFile(planPath, []byte("# Retry Exhausted"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Still bad task",
						"description": "Too many criteria",
						"priority": "P1",
						"acceptance_criteria": ["a", "b", "c", "d"],
						"depends_on_index": []
					},
					{
						"title": "Another task",
						"description": "Also bad",
						"priority": "P1",
						"acceptance_criteria": ["w", "x", "y", "z"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "retry-exhausted",
		MaxValidationRetries: 1,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}
	if runCount != 2 {
		t.Fatalf("Run() call count = %d, want 2", runCount)
	}

	// Verify ValidationStats when retries exhausted without improvement
	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if result.ValidationStats.Attempts != 2 {
		t.Errorf("ValidationStats.Attempts = %d, want 2", result.ValidationStats.Attempts)
	}
	// 2 violations per attempt (2 bad beads × 1 criteria_count each) × 2 attempts = 4 total
	if result.ValidationStats.ViolationCount != 4 {
		t.Errorf("ValidationStats.ViolationCount = %d, want 4", result.ValidationStats.ViolationCount)
	}
	if result.ValidationStats.Improved {
		t.Error("ValidationStats.Improved = true, want false (violations unchanged)")
	}
}

func TestDecomposeWorkflow_SkipValidationDisablesRetryLoop(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "skip-validation.md")
	if err := os.WriteFile(planPath, []byte("# Skip Validation"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Bad task",
						"description": "Too many criteria",
						"priority": "P1",
						"acceptance_criteria": ["a", "b", "c", "d"],
						"depends_on_index": []
					},
					{
						"title": "Another task",
						"description": "Also bad",
						"priority": "P1",
						"acceptance_criteria": ["w", "x", "y", "z"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "skip-validation",
		SkipValidation:       true,
		MaxValidationRetries: 4,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}
	if runCount != 1 {
		t.Fatalf("Run() call count = %d, want 1", runCount)
	}
}

// TestDecomposeWorkflow_CreatesBeadsWithCorrectLabels verifies beads get spec:<name> label
// Expected failure: Pipeline.Decompose() does not add spec label to created beads
func TestDecomposeWorkflow_CreatesBeadsWithCorrectLabels(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "caching.md")
	if err := os.WriteFile(planPath, []byte("# Caching Plan\n\nImplement caching layer"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Simple task",
						"description": "A simple supporting task",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Add cache interface",
						"description": "Define cache interface",
						"priority": "P1",
						"estimated_files": 6,
						"acceptance_criteria": ["Interface defined"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	var capturedLabels []string
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedLabels = labels
			return &BeadInfo{ID: "bead-1", Title: title, Priority: priority, Labels: labels}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "caching",
	}

	_, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if !containsString(capturedLabels, "spec:caching") {
		t.Errorf("Labels = %v, want to include 'spec:caching'", capturedLabels)
	}
	if !containsString(capturedLabels, "complexity:high") {
		t.Errorf("Labels = %v, want to include 'complexity:high'", capturedLabels)
	}
	if !containsString(capturedLabels, "estimated-files:6") {
		t.Errorf("Labels = %v, want to include 'estimated-files:6'", capturedLabels)
	}
}

// TestDecomposeWorkflow_HandlesDependencyMapping verifies dependency index resolution
// Expected failure: Pipeline.Decompose() does not map depends_on_index to actual bead IDs
func TestDecomposeWorkflow_HandlesDependencyMapping(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan\n\nWith dependencies"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			// Return 3 beads with dependency chain: bead2 depends on bead0, bead1 has no deps
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Task A",
						"description": "First task",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Task B",
						"description": "Independent task",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Task C",
						"description": "Depends on Task A",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": [0]
					}
				]`,
			}, nil
		},
	}

	type beadCapture struct {
		title string
		deps  []string
	}
	var capturedBeads []beadCapture
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedBeads = append(capturedBeads, beadCapture{title: title, deps: deps})
			return &BeadInfo{
				ID:       fmt.Sprintf("bead-%d", len(capturedBeads)),
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify dependency mapping
	if len(capturedBeads) != 3 {
		t.Fatalf("expected 3 beads, got %d", len(capturedBeads))
	}

	// Task A (index 0) - no dependencies
	if len(capturedBeads[0].deps) != 0 {
		t.Errorf("Task A deps = %v, want empty", capturedBeads[0].deps)
	}

	// Task B (index 1) - no dependencies
	if len(capturedBeads[1].deps) != 0 {
		t.Errorf("Task B deps = %v, want empty", capturedBeads[1].deps)
	}

	// Task C (index 2) - depends on bead-1 (Task A)
	if len(capturedBeads[2].deps) != 1 {
		t.Fatalf("Task C deps = %v, want 1 dependency", capturedBeads[2].deps)
	}
	if capturedBeads[2].deps[0] != "bead-1" {
		t.Errorf("Task C deps[0] = %q, want 'bead-1' (Task A's ID)", capturedBeads[2].deps[0])
	}
}

// TestDecomposeWorkflow_SkipsSelfDependencies verifies self-dependencies are skipped with warning
// Expected failure: Pipeline.Decompose() does not detect and skip self-dependencies
func TestDecomposeWorkflow_SkipsSelfDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			// Return 2 beads: second has self-dependency (index 1 depends on index 1)
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Normal task",
						"description": "No dependency",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Task with self-dep",
						"description": "Bad dependency",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": [1]
					}
				]`,
			}, nil
		},
	}

	var capturedDeps []string
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedDeps = deps
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify self-dependency was skipped (empty deps list)
	if len(capturedDeps) != 0 {
		t.Errorf("Dependencies = %v, want empty (self-dependency should be skipped)", capturedDeps)
	}
}

// TestDecomposeWorkflow_SkipsOutOfRangeDependencies verifies invalid dependency indices are skipped
// Expected failure: Pipeline.Decompose() does not validate dependency index range
func TestDecomposeWorkflow_SkipsOutOfRangeDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			// Return 2 beads, second depends on index 5 (out of range)
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Task A",
						"description": "First",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Task B",
						"description": "Bad dep",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": [5]
					}
				]`,
			}, nil
		},
	}

	type beadCapture struct {
		title string
		deps  []string
	}
	var capturedBeads []beadCapture
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedBeads = append(capturedBeads, beadCapture{title: title, deps: deps})
			return &BeadInfo{ID: fmt.Sprintf("bead-%d", len(capturedBeads))}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify Task B has no dependencies (out-of-range dep was skipped)
	if len(capturedBeads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(capturedBeads))
	}
	if len(capturedBeads[1].deps) != 0 {
		t.Errorf("Task B deps = %v, want empty (out-of-range dependency should be skipped)", capturedBeads[1].deps)
	}
}

// TestDecomposeWorkflow_ReviewModeReturnsProposedBeads verifies Review=true returns beads without creating
// Expected failure: Pipeline.Decompose() does not support Review mode
func TestDecomposeWorkflow_ReviewModeReturnsProposedBeads(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Test task",
						"description": "Test",
						"priority": "P1",
						"estimated_files": 2,
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Another task",
						"description": "Supporting work",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	beadCreateCalled := false
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			beadCreateCalled = true
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
		Review:   true, // Review mode
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify beads are returned but not created
	if len(result.CreatedBeads) != 2 {
		t.Errorf("CreatedBeads count = %d, want 2 (proposed beads)", len(result.CreatedBeads))
	}
	if !containsString(result.CreatedBeads[0].Labels, "estimated-files:2") {
		t.Errorf("Review labels = %v, want to include 'estimated-files:2'", result.CreatedBeads[0].Labels)
	}

	// Verify bead client was NOT called
	if beadCreateCalled {
		t.Error("BeadClient.Create() was called in review mode, should return proposed beads without creating")
	}

	// Verify plan was NOT updated in review mode
	if result.PlanUpdated {
		t.Error("PlanUpdated = true in review mode, want false")
	}
}

// TestDecomposeWorkflow_ForceRedecomposesExistingPlan verifies Force=true allows re-decomposition
// Expected failure: Pipeline.Decompose() does not check decomposed status or respect Force flag
func TestDecomposeWorkflow_ForceRedecomposesExistingPlan(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan already marked as decomposed
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	planContent := `---
spec: test
decomposed: true
decomposed_at: 2026-02-11T10:00:00Z
---

# Test Plan

Already decomposed
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "New task",
						"description": "From redecompose",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Supporting task",
						"description": "From redecompose",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()

	// First attempt WITHOUT force - should fail
	inputNoForce := DecomposeInput{
		PlanName: "test-plan",
		Force:    false,
	}

	_, err := p.Decompose(ctx, inputNoForce)
	if err == nil {
		t.Error("Decompose() without force should fail when plan already decomposed, got nil error")
	}
	if err != nil && !strings.Contains(err.Error(), "already decomposed") && !strings.Contains(err.Error(), "decomposed") {
		t.Errorf("Expected 'already decomposed' error, got: %v", err)
	}

	// Second attempt WITH force - should succeed
	inputWithForce := DecomposeInput{
		PlanName: "test-plan",
		Force:    true,
	}

	result, err := p.Decompose(ctx, inputWithForce)
	if err != nil {
		t.Fatalf("Decompose() with force=true failed: %v (should allow re-decomposition)", err)
	}

	if len(result.CreatedBeads) != 2 {
		t.Errorf("Force re-decompose created %d beads, want 2", len(result.CreatedBeads))
	}
}

// TestDecomposeWorkflow_PlanNotFoundError verifies error when plan doesn't exist
// Expected failure: Pipeline.Decompose() does not validate plan file existence
func TestDecomposeWorkflow_PlanNotFoundError(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Don't create the plan file
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	deps := &Deps{
		ClaudeClient: &decomposeAcceptanceClaudeClient{},
		BeadClient:   &decomposeAcceptanceBeadClient{},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "nonexistent-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Fatal("Decompose() should fail for nonexistent plan, got nil error")
	}

	if !strings.Contains(err.Error(), "plan not found") && !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'plan not found' error, got: %v", err)
	}
}

// TestDecomposeWorkflow_UpdatesPlanFrontmatterTimestamp verifies decomposed_at timestamp is set
// Expected failure: Pipeline.Decompose() does not set decomposed_at timestamp in frontmatter
func TestDecomposeWorkflow_UpdatesPlanFrontmatterTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan\n\nContent"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Task",
						"description": "Test",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Supporting task",
						"description": "Test",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	beforeTime := time.Now()
	_, err := p.Decompose(ctx, input)
	afterTime := time.Now()

	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Read plan and verify timestamp
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading plan file: %v", err)
	}
	planStr := string(planData)

	// Extract timestamp value
	if !strings.Contains(planStr, "decomposed_at:") {
		t.Fatal("Plan frontmatter missing decomposed_at field")
	}

	// Verify timestamp is in reasonable range (between before and after)
	// This is a basic check - just verify the field exists and looks like a timestamp
	if !strings.Contains(planStr, "2026-02-") {
		t.Error("decomposed_at timestamp does not appear to be current date")
	}

	// More rigorous: could parse the timestamp and verify it's within range
	_ = beforeTime
	_ = afterTime
}

// TestDecomposeWorkflow_ParsesPriorityCorrectly verifies priority string to int conversion
// Expected failure: Pipeline.Decompose() does not parse priority strings correctly
func TestDecomposeWorkflow_ParsesPriorityCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	// Create plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "High priority task",
						"description": "P0",
						"priority": "P0",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Medium priority task",
						"description": "P1",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Low priority task",
						"description": "P2",
						"priority": "P2",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	type beadCapture struct {
		title    string
		priority int
	}
	var capturedBeads []beadCapture
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedBeads = append(capturedBeads, beadCapture{title: title, priority: priority})
			return &BeadInfo{ID: fmt.Sprintf("bead-%d", len(capturedBeads))}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify priority mappings: P0->0, P1->1, P2->2
	if len(capturedBeads) != 3 {
		t.Fatalf("expected 3 beads, got %d", len(capturedBeads))
	}

	if capturedBeads[0].priority != 0 {
		t.Errorf("P0 task priority = %d, want 0", capturedBeads[0].priority)
	}
	if capturedBeads[1].priority != 1 {
		t.Errorf("P1 task priority = %d, want 1", capturedBeads[1].priority)
	}
	if capturedBeads[2].priority != 2 {
		t.Errorf("P2 task priority = %d, want 2", capturedBeads[2].priority)
	}
}

// TestDecomposeWorkflow_NilDependenciesError verifies error for nil dependencies
// Expected failure: Pipeline.Decompose() does not properly validate nil dependencies
func TestDecomposeWorkflow_NilDependenciesError(t *testing.T) {
	p := New(nil, &Paths{})

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test",
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Error("Decompose() with nil dependencies returned nil error, want error")
	}

	if !strings.Contains(err.Error(), "nil dependencies") && !strings.Contains(err.Error(), "dependencies") {
		t.Errorf("Error message = %q, want message about nil dependencies", err.Error())
	}
}

func TestBuildDecomposePrompt_ConstructsPromptDiagnostics(t *testing.T) {
	planName := "authentication"
	planBody := "# Plan\n\nImplement auth flow"
	skillContent := "Skill guidance"

	promptText, diagnostics := buildDecomposePrompt(planName, planBody, skillContent)
	if !strings.Contains(promptText, planBody) {
		t.Fatalf("prompt missing plan body content")
	}
	if !strings.Contains(promptText, skillContent) {
		t.Fatalf("prompt missing skill instructions content")
	}

	if diagnostics == nil {
		t.Fatal("diagnostics = nil, want non-nil")
	}
	if diagnostics.PromptType != decomposePromptType {
		t.Fatalf("PromptType = %q, want %q", diagnostics.PromptType, decomposePromptType)
	}

	templateStatic := decomposeTemplateStatic(planName)
	wantSectionTokens := map[string]int{
		prompt.SectionPlanBody:          prompt.EstimateTokens(planBody),
		prompt.SectionSkillInstructions: prompt.EstimateTokens(skillContent),
		prompt.SectionTemplateStatic:    prompt.EstimateTokens(templateStatic),
	}
	for section, want := range wantSectionTokens {
		if got := diagnostics.SectionTokens[section]; got != want {
			t.Errorf("SectionTokens[%q] = %d, want %d", section, got, want)
		}
	}

	wantEstimated := wantSectionTokens[prompt.SectionPlanBody] + wantSectionTokens[prompt.SectionSkillInstructions] + wantSectionTokens[prompt.SectionTemplateStatic]
	if diagnostics.EstimatedTokens != wantEstimated {
		t.Errorf("EstimatedTokens = %d, want %d", diagnostics.EstimatedTokens, wantEstimated)
	}
}

// Mock types for acceptance tests

type decomposeAcceptanceClaudeClient struct {
	runFunc func(prompt string, model string) (*ClaudeRunResult, error)
}

func (m *decomposeAcceptanceClaudeClient) Run(prompt string, model string) (*ClaudeRunResult, error) {
	if m.runFunc != nil {
		return m.runFunc(prompt, model)
	}
	return nil, fmt.Errorf("not implemented")
}

type decomposeAcceptanceBeadClient struct {
	createFunc func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error)
}

func (m *decomposeAcceptanceBeadClient) Ready() (*BeadInfo, error) {
	return nil, nil
}

func (m *decomposeAcceptanceBeadClient) Show(id string) (*BeadInfo, error) {
	return nil, nil
}

func (m *decomposeAcceptanceBeadClient) Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
	// Decompose workflow should use CreateWithDepsAndDescription, not Create
	// This is here to satisfy the BeadClient interface
	return nil, fmt.Errorf("decompose should use CreateWithDepsAndDescription")
}

func (m *decomposeAcceptanceBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
	if m.createFunc != nil {
		return m.createFunc(title, priority, labels, criteria, deps, desc)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *decomposeAcceptanceBeadClient) Close(id string) error {
	return nil
}

// TestDecomposeWorkflow_UsesExpectedOutputsWhenNonEmpty verifies that when expected_outputs is non-empty,
// it is passed to CreateWithDepsAndDescription instead of acceptance_criteria.
func TestDecomposeWorkflow_UsesExpectedOutputsWhenNonEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "test-plan.md"), []byte("# Test Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Simple prerequisite",
						"description": "No expected outputs",
						"priority": "P1",
						"acceptance_criteria": ["Setup done"],
						"depends_on_index": []
					},
					{
						"title": "Task with outputs",
						"description": "Has expected outputs",
						"priority": "P1",
						"acceptance_criteria": ["Criterion A"],
						"expected_outputs": ["Output X", "Output Y"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	var capturedCriteria []string
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedCriteria = criteria
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{PlanName: "test-plan"})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if len(capturedCriteria) != 2 {
		t.Fatalf("criteria passed to CreateWithDepsAndDescription length = %d, want 2 (from expected_outputs)", len(capturedCriteria))
	}
	if capturedCriteria[0] != "Output X" {
		t.Errorf("criteria[0] = %q, want %q", capturedCriteria[0], "Output X")
	}
	if capturedCriteria[1] != "Output Y" {
		t.Errorf("criteria[1] = %q, want %q", capturedCriteria[1], "Output Y")
	}
}

// TestDecomposeWorkflow_FallsBackToAcceptanceCriteriaWhenNoExpectedOutputs verifies that when
// expected_outputs is empty, acceptance_criteria is used.
func TestDecomposeWorkflow_FallsBackToAcceptanceCriteriaWhenNoExpectedOutputs(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "test-plan.md"), []byte("# Test Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Simple prereq",
						"description": "No expected outputs",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Task without outputs",
						"description": "Has no expected outputs",
						"priority": "P1",
						"acceptance_criteria": ["Criterion A", "Criterion B"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	var capturedCriteria []string
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			capturedCriteria = criteria
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{PlanName: "test-plan"})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if len(capturedCriteria) != 2 {
		t.Fatalf("criteria passed to CreateWithDepsAndDescription length = %d, want 2 (from acceptance_criteria)", len(capturedCriteria))
	}
	if capturedCriteria[0] != "Criterion A" {
		t.Errorf("criteria[0] = %q, want %q", capturedCriteria[0], "Criterion A")
	}
	if capturedCriteria[1] != "Criterion B" {
		t.Errorf("criteria[1] = %q, want %q", capturedCriteria[1], "Criterion B")
	}
}

// TestBeadDef_ExpectedOutputsDeserializesFromJSON verifies expected_outputs JSON field is parsed correctly.
func TestBeadDef_ExpectedOutputsDeserializesFromJSON(t *testing.T) {
	jsonInput := `{
		"title": "Test task",
		"description": "A task",
		"priority": "P1",
		"acceptance_criteria": ["Criterion A"],
		"expected_outputs": ["Output X", "Output Y"],
		"depends_on_index": []
	}`

	var def beadDef
	if err := json.Unmarshal([]byte(jsonInput), &def); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	if len(def.ExpectedOutputs) != 2 {
		t.Fatalf("ExpectedOutputs length = %d, want 2", len(def.ExpectedOutputs))
	}
	if def.ExpectedOutputs[0] != "Output X" {
		t.Errorf("ExpectedOutputs[0] = %q, want %q", def.ExpectedOutputs[0], "Output X")
	}
	if def.ExpectedOutputs[1] != "Output Y" {
		t.Errorf("ExpectedOutputs[1] = %q, want %q", def.ExpectedOutputs[1], "Output Y")
	}
}

// TestDecomposePrompt_MentionsExpectedOutputs verifies the decompose prompt instructs
// the LLM to populate expected_outputs with fine-grained deliverables for TDD cycles.
func TestDecomposePrompt_MentionsExpectedOutputs(t *testing.T) {
	promptText, _ := buildDecomposePrompt("test-plan", "# Plan body", "Skill content")
	promptLower := strings.ToLower(promptText)

	if !strings.Contains(promptLower, "expected_outputs") {
		t.Errorf("decompose prompt should mention expected_outputs field.\nPrompt:\n%s", promptText)
	}
}

func TestDecomposeWorkflow_UsesInputTierForProviderCall(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "tier-plan.md"), []byte("# Tier Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	gotModel := ""
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			gotModel = model
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{
						"title": "Task",
						"description": "Desc",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					},
					{
						"title": "Supporting task",
						"description": "Desc",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName: "tier-plan",
		Tier:     "high",
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if gotModel != "opus" {
		t.Errorf("provider model = %q, want %q for input tier high", gotModel, "opus")
	}
}

// TestDecompose_SixBeads_TruncatesToFive verifies that when the model returns 6 sub-beads,
// the batch_size_max fallback truncates the result to 5 beads before creation.
func TestDecompose_SixBeads_TruncatesToFive(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "big-plan.md"), []byte("# Big Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Task 1","description":"d","priority":"P1","acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Task 2","description":"d","priority":"P1","acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Task 3","description":"d","priority":"P1","acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Task 4","description":"d","priority":"P1","acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Task 5","description":"d","priority":"P1","acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Task 6","description":"d","priority":"P1","acceptance_criteria":["a"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	var createdCount int
	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			createdCount++
			return &BeadInfo{ID: fmt.Sprintf("bead-%d", createdCount)}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "big-plan",
		MaxValidationRetries: 0,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}
	if len(result.CreatedBeads) != 5 {
		t.Errorf("CreatedBeads count = %d, want 5 (truncated from 6 by batch_size_max fallback)", len(result.CreatedBeads))
	}
	if createdCount != 5 {
		t.Errorf("bead create calls = %d, want 5 (only 5 beads should be created)", createdCount)
	}
}

func TestDecomposeWorkflow_LogsComplexitySummaryPerAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-plan.md"), []byte("# Complexity Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Low task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"High task","description":"d","priority":"P1","estimated_files":7,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})

	output := captureStdout(t, func() {
		_, err := p.Decompose(context.Background(), DecomposeInput{PlanName: "complexity-plan"})
		if err != nil {
			t.Fatalf("Decompose() failed: %v", err)
		}
	})

	if !strings.Contains(output, "Complexity summary (attempt 1): high=1 low=1") {
		t.Fatalf("stdout missing complexity summary line, got:\n%s", output)
	}
}

func TestDecomposeWorkflow_IncludesComplexityFeedbackInReprompt(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "reprompt-complexity.md"), []byte("# Reprompt Complexity"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	prompts := []string{}
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			prompts = append(prompts, prompt)
			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Overly broad task","description":"d","priority":"P1","estimated_files":9,"acceptance_criteria":["a","b","c","d"],"depends_on_index":[]},
						{"title":"Supporting task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["x"],"depends_on_index":[]}
					]`,
				}, nil
			}

			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Narrow task","description":"d","priority":"P1","estimated_files":2,"acceptance_criteria":["a","b"],"depends_on_index":[]},
					{"title":"Supporting task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["x"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "reprompt-complexity",
		MaxValidationRetries: 2,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}
	if runCount != 2 {
		t.Fatalf("Run() call count = %d, want 2", runCount)
	}

	secondPrompt := prompts[1]
	if !strings.Contains(secondPrompt, "Complexity feedback:") {
		t.Fatalf("second prompt missing complexity feedback section, got:\n%s", secondPrompt)
	}
	if !strings.Contains(secondPrompt, "Overly broad task") {
		t.Fatalf("second prompt missing high-complexity bead title, got:\n%s", secondPrompt)
	}
}

func TestDecomposeWorkflow_IncludesStructuredComplexityFeedbackAndStillCreatesBeadsAfterLoopExit(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "reprompt-structured-complexity.md"), []byte("# Reprompt Structured Complexity"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	prompts := []string{}
	createCalls := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			prompts = append(prompts, prompt)
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Overly broad task","description":"d","priority":"P1","estimated_files":9,"acceptance_criteria":["a","b","c"],"depends_on_index":[]},
					{"title":"Supporting task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["x"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			createCalls++
			return &BeadInfo{ID: fmt.Sprintf("bead-%d", createCalls)}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "reprompt-structured-complexity",
		MaxValidationRetries: 1,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}
	if runCount != 2 {
		t.Fatalf("Run() call count = %d, want 2", runCount)
	}

	secondPrompt := prompts[1]
	if !strings.Contains(secondPrompt, "## Complexity Feedback") {
		t.Fatalf("second prompt missing structured complexity feedback section, got:\n%s", secondPrompt)
	}
	if !strings.Contains(secondPrompt, "Overly broad task") {
		t.Fatalf("second prompt missing high-complexity bead title, got:\n%s", secondPrompt)
	}
	if createCalls != 2 {
		t.Fatalf("CreateWithDepsAndDescription call count = %d, want 2 (bead creation should proceed after loop exit)", createCalls)
	}
}

func TestDecomposeWorkflow_RetriesWhenHighComplexityRemains(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-retry.md"), []byte("# Complexity Retry"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	var secondPrompt string
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			if runCount == 2 {
				secondPrompt = prompt
			}
			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"High task","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["a"],"depends_on_index":[]},
						{"title":"Low task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
					]`,
				}, nil
			}

			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Reduced task","description":"d","priority":"P1","estimated_files":3,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Low task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "complexity-retry",
		MaxValidationRetries: 2,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if runCount != 2 {
		t.Fatalf("Run() call count = %d, want 2", runCount)
	}
	if !strings.Contains(secondPrompt, "Complexity feedback:") {
		t.Fatalf("second prompt missing complexity feedback, got:\n%s", secondPrompt)
	}
}

func TestDecomposeWorkflow_HighComplexityWarningIncludesDetails(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-warning.md"), []byte("# Complexity Warning"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Overly broad API task","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})

	output := captureStdout(t, func() {
		_, err := p.Decompose(context.Background(), DecomposeInput{
			PlanName:             "complexity-warning",
			MaxValidationRetries: 0,
		})
		if err != nil {
			t.Fatalf("Decompose() failed: %v", err)
		}
	})

	if !strings.Contains(output, "Warning: high-complexity beads remain") {
		t.Fatalf("stdout missing high-complexity warning, got:\n%s", output)
	}
	if !strings.Contains(output, "remaining=1") {
		t.Fatalf("stdout missing remaining high-complexity count detail, got:\n%s", output)
	}
	if !strings.Contains(output, "Overly broad API task") {
		t.Fatalf("stdout missing high-complexity title detail, got:\n%s", output)
	}
	if !strings.Contains(output, "estimated_files=8") {
		t.Fatalf("stdout missing high-complexity reason snippet, got:\n%s", output)
	}
}

func TestDecomposeWorkflow_FinalHighComplexityWarningAtLoopExitAfterValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-final-warning.md"), []byte("# Complexity Final Warning"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Refactor entire authentication system","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})

	output := captureStdout(t, func() {
		_, err := p.Decompose(context.Background(), DecomposeInput{
			PlanName:             "complexity-final-warning",
			MaxValidationRetries: 0,
		})
		if err != nil {
			t.Fatalf("Decompose() failed: %v", err)
		}
	})

	if !strings.Contains(output, "Warning: high-complexity beads remain") {
		t.Fatalf("stdout missing final high-complexity warning, got:\n%s", output)
	}
	if !strings.Contains(output, "remaining=1") {
		t.Fatalf("stdout missing remaining high-complexity count detail, got:\n%s", output)
	}
	if !strings.Contains(output, "Refactor entire authentication system") {
		t.Fatalf("stdout missing high-complexity title detail, got:\n%s", output)
	}
	if !strings.Contains(output, "estimated_files=8") {
		t.Fatalf("stdout missing high-complexity reason snippet, got:\n%s", output)
	}
}

func TestDecomposeWorkflow_ComplexitySummaryIncludesHighTitles(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-summary.md"), []byte("# Complexity Summary"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Broad migration task","description":"d","priority":"P1","estimated_files":7,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})

	output := captureStdout(t, func() {
		_, err := p.Decompose(context.Background(), DecomposeInput{
			PlanName:             "complexity-summary",
			MaxValidationRetries: 0,
		})
		if err != nil {
			t.Fatalf("Decompose() failed: %v", err)
		}
	})

	if !strings.Contains(output, "Complexity summary (attempt 1): high=1 low=1") {
		t.Fatalf("stdout missing complexity summary line, got:\n%s", output)
	}
	if !strings.Contains(output, "high_titles=[Broad migration task]") {
		t.Fatalf("stdout missing high-complexity titles in summary, got:\n%s", output)
	}
}

func TestDecomposeWorkflow_UsesScoreBasedComplexitySummaryAndReasons(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "score-based-complexity.md"), []byte("# Score Based Complexity"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Refactor entire auth workflow","description":"touches multiple areas","priority":"P1","estimated_files":1,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})

	output := captureStdout(t, func() {
		_, err := p.Decompose(context.Background(), DecomposeInput{
			PlanName:             "score-based-complexity",
			MaxValidationRetries: 0,
		})
		if err != nil {
			t.Fatalf("Decompose() failed: %v", err)
		}
	})

	if !strings.Contains(output, "Complexity summary (attempt 1): high=1 low=1") {
		t.Fatalf("stdout missing score-based complexity summary counts, got:\n%s", output)
	}
	if !strings.Contains(output, "contains broad-scope language in title or description") {
		t.Fatalf("stdout missing score-based complexity reason in summary, got:\n%s", output)
	}
}

func TestFormatComplexitySummaryLine_UsesAttemptNumberAndCounts(t *testing.T) {
	line := formatComplexitySummaryLine(2, validate.ValidateDecomposeCandidates([]validate.BeadCandidate{
		{Title: "Big task", EstimatedFiles: 7},
		{Title: "Small task", EstimatedFiles: 1},
	}))

	if line != "Complexity summary (attempt 2): high=1 low=1 high_titles=[Big task]\n" {
		t.Fatalf("summary line = %q", line)
	}
}

func TestDecomposeWorkflow_CleanExitAfterComplexityRetryHasNoWarning(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-clean-exit.md"), []byte("# Complexity Clean Exit"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Large task","description":"d","priority":"P1","estimated_files":9,"acceptance_criteria":["a"],"depends_on_index":[]},
						{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
					]`,
				}, nil
			}
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Split task","description":"d","priority":"P1","estimated_files":2,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	output := captureStdout(t, func() {
		_, err := p.Decompose(context.Background(), DecomposeInput{
			PlanName:             "complexity-clean-exit",
			MaxValidationRetries: 2,
		})
		if err != nil {
			t.Fatalf("Decompose() failed: %v", err)
		}
	})

	if strings.Contains(output, "Warning: high-complexity beads remain") {
		t.Fatalf("stdout should not include high-complexity warning on clean exit, got:\n%s", output)
	}
	if !strings.Contains(output, "Complexity clean exit after attempt 2: no high-complexity warning emitted.") {
		t.Fatalf("stdout missing explicit clean-exit message, got:\n%s", output)
	}
}

func TestDecomposeWorkflow_ComplexityRetryCapWithPartialImprovementMarksImproved(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-partial-improvement.md"), []byte("# Complexity Partial Improvement"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Very broad task A","description":"d","priority":"P1","estimated_files":9,"acceptance_criteria":["a"],"depends_on_index":[]},
						{"title":"Very broad task B","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["b"],"depends_on_index":[]},
						{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["c"],"depends_on_index":[]}
					]`,
				}, nil
			}
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Split task A","description":"d","priority":"P1","estimated_files":3,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Still broad task B","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["b"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["c"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "complexity-partial-improvement",
		MaxValidationRetries: 1,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if runCount != 2 {
		t.Fatalf("Run() call count = %d, want 2", runCount)
	}
	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if result.ValidationStats.Attempts != 2 {
		t.Fatalf("ValidationStats.Attempts = %d, want 2", result.ValidationStats.Attempts)
	}
	if !result.ValidationStats.Improved {
		t.Fatal("ValidationStats.Improved = false, want true when high-complexity count decreases before retry cap")
	}
}

func TestDecomposeWorkflow_StopsRetryingWhenComplexityTrajectoryStalls(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-stall-stop.md"), []byte("# Complexity Stall Stop"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			switch runCount {
			case 1:
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Very broad task A","description":"d","priority":"P1","estimated_files":9,"acceptance_criteria":["a"],"depends_on_index":[]},
						{"title":"Very broad task B","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["b"],"depends_on_index":[]},
						{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["c"],"depends_on_index":[]}
					]`,
				}, nil
			case 2:
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Split task A","description":"d","priority":"P1","estimated_files":3,"acceptance_criteria":["a"],"depends_on_index":[]},
						{"title":"Still broad task B","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["b"],"depends_on_index":[]},
						{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["c"],"depends_on_index":[]}
					]`,
				}, nil
			default:
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Split task A","description":"d","priority":"P1","estimated_files":3,"acceptance_criteria":["a"],"depends_on_index":[]},
						{"title":"Still broad task B","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["b"],"depends_on_index":[]},
						{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["c"],"depends_on_index":[]}
					]`,
				}, nil
			}
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	_, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "complexity-stall-stop",
		MaxValidationRetries: 4,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if runCount != 3 {
		t.Fatalf("Run() call count = %d, want 3 (stop once high-complexity count stops improving)", runCount)
	}
}

func TestDecomposeWorkflow_HighComplexityWarningProceedSetsValidationFlag(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "complexity-warning-flag.md"), []byte("# Complexity Warning Flag"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Still too broad","description":"d","priority":"P1","estimated_files":8,"acceptance_criteria":["a"],"depends_on_index":[]},
					{"title":"Small task","description":"d","priority":"P1","estimated_files":1,"acceptance_criteria":["b"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "complexity-warning-flag",
		MaxValidationRetries: 0,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if !result.ValidationStats.ProceededWithHighComplexityWarning {
		t.Fatal("ValidationStats.ProceededWithHighComplexityWarning = false, want true on warning proceed path")
	}
}

func TestDecomposeWorkflow_RetryCapPathSetsValidationFlag(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "retry-cap-flag.md"), []byte("# Retry Cap Flag"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Still invalid task","description":"d","priority":"P1","acceptance_criteria":["a","b","c","d"],"depends_on_index":[]},
					{"title":"Another invalid task","description":"d","priority":"P1","acceptance_criteria":["w","x","y","z"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "retry-cap-flag",
		MaxValidationRetries: 1,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if !result.ValidationStats.RetryCapReached {
		t.Fatal("ValidationStats.RetryCapReached = false, want true when decompose exits at retry cap")
	}
}

func TestDecomposeWorkflow_RetryLoopSuccessPathSetsValidationFlag(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "retry-success-flag.md"), []byte("# Retry Success Flag"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Needs fix","description":"d","priority":"P1","acceptance_criteria":["a","b","c","d"],"depends_on_index":[]},
						{"title":"Good task","description":"d","priority":"P1","acceptance_criteria":["x","y"],"depends_on_index":[]}
					]`,
				}, nil
			}
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Fixed","description":"d","priority":"P1","acceptance_criteria":["a","b","c"],"depends_on_index":[]},
					{"title":"Good task","description":"d","priority":"P1","acceptance_criteria":["x","y"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "retry-success-flag",
		MaxValidationRetries: 2,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if !result.ValidationStats.SucceededAfterRetry {
		t.Fatal("ValidationStats.SucceededAfterRetry = false, want true for retry-loop success path")
	}
	if result.ValidationStats.RetryCapReached {
		t.Fatal("ValidationStats.RetryCapReached = true, want false for retry-loop success path")
	}
}

func TestDecomposeWorkflow_ValidationFixCountsAsSuccessEvenWithHighComplexityWarning(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "validation-fix-with-high-complexity.md"), []byte("# Validation Fix High Complexity"), 0644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			runCount++
			if runCount == 1 {
				return &ClaudeRunResult{
					Success:  true,
					ExitCode: 0,
					Output: `[
						{"title":"Needs fix","description":"d","priority":"P1","acceptance_criteria":["a","b","c","d"],"depends_on_index":[]},
						{"title":"Small valid task","description":"d","priority":"P1","acceptance_criteria":["x"],"depends_on_index":[]}
					]`,
				}, nil
			}
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
						{"title":"Fixed but still broad","description":"d","priority":"P1","acceptance_criteria":["a","b","c"],"depends_on_index":[]},
						{"title":"Small valid task","description":"d","priority":"P1","acceptance_criteria":["x"],"depends_on_index":[]}
					]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "validation-fix-with-high-complexity",
		MaxValidationRetries: 2,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if !result.ValidationStats.Improved {
		t.Fatal("ValidationStats.Improved = false, want true when validation violations are resolved")
	}
	if !result.ValidationStats.SucceededAfterRetry {
		t.Fatal("ValidationStats.SucceededAfterRetry = false, want true when retry resolves validation violations")
	}
	if !result.ValidationStats.ProceededWithHighComplexityWarning {
		t.Fatal("ValidationStats.ProceededWithHighComplexityWarning = false, want true when high-complexity warning path is used")
	}
}

func TestDecomposeWorkflow_RetryLoopNonImprovingPathSetsValidationFlag(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "retry-non-improving-flag.md"), []byte("# Retry Non Improving Flag"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeAcceptanceClaudeClient{
		runFunc: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output: `[
					{"title":"Still invalid task","description":"d","priority":"P1","acceptance_criteria":["a","b","c","d"],"depends_on_index":[]},
					{"title":"Another invalid task","description":"d","priority":"P1","acceptance_criteria":["w","x","y","z"],"depends_on_index":[]}
				]`,
			}, nil
		},
	}

	mockBead := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	p := New(&Deps{ClaudeClient: mockClaude, BeadClient: mockBead}, &Paths{PlansDir: plansDir})
	result, err := p.Decompose(context.Background(), DecomposeInput{
		PlanName:             "retry-non-improving-flag",
		MaxValidationRetries: 1,
	})
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if result.ValidationStats == nil {
		t.Fatal("ValidationStats = nil, want populated")
	}
	if !result.ValidationStats.NonImprovingAtRetryCap {
		t.Fatal("ValidationStats.NonImprovingAtRetryCap = false, want true for non-improving retry-cap path")
	}
	if result.ValidationStats.Improved {
		t.Fatal("ValidationStats.Improved = true, want false for non-improving retry-cap path")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return buf.String()
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
