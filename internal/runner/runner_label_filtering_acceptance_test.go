package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
)

// TestRunnerLabelFiltering_NoLabels verifies that when no label filters are set,
// the runner uses Ready() and processes all beads by priority (current behavior)
func TestRunnerLabelFiltering_NoLabels(t *testing.T) {
	var output strings.Builder

	allBeads := []*bead.Bead{
		{ID: "bead-1", Title: "First task", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "bead-2", Title: "Second task", Priority: 2, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
		{ID: "bead-3", Title: "Third task", Priority: 0, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
	}
	currentBead := 0
	var processedBeads []string

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if currentBead >= len(allBeads) {
				return nil, nil // No more work
			}
			b := allBeads[currentBead]
			currentBead++
			return b, nil
		},
		CloseFn: func(id string) error {
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify all beads were processed (no label filtering)
	if len(processedBeads) != 3 {
		t.Errorf("Expected 3 beads to be processed, got %d: %v", len(processedBeads), processedBeads)
	}

	// Verify Ready() was called (not ReadyWithLabel)
	// This is implicit - if ReadyWithLabel was called, the mock would have failed
}

// TestRunnerLabelFiltering_SingleLabel verifies that when a single label filter is set,
// the runner only processes beads with that label using ReadyWithLabel()
func TestRunnerLabelFiltering_SingleLabel(t *testing.T) {
	var output strings.Builder

	authBeads := []*bead.Bead{
		{ID: "auth-1", Title: "Auth task 1", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "auth-2", Title: "Auth task 2", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
	}
	authIndex := 0
	var processedBeads []string
	var readyWithLabelCalls []string

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			t.Error("Ready() should not be called when label filters are set")
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalls = append(readyWithLabelCalls, label)
			if label == "spec:auth" && authIndex < len(authBeads) {
				b := authBeads[authIndex]
				authIndex++
				return b, nil
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set label filters - this method should exist after implementation
	r.SetLabelFilters([]string{"spec:auth"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify only auth beads were processed
	if len(processedBeads) != 2 {
		t.Errorf("Expected 2 auth beads to be processed, got %d: %v", len(processedBeads), processedBeads)
	}

	// Verify ReadyWithLabel was called with "spec:auth"
	if len(readyWithLabelCalls) == 0 {
		t.Error("Expected ReadyWithLabel to be called, but it wasn't")
	}
	for _, label := range readyWithLabelCalls {
		if label != "spec:auth" {
			t.Errorf("Expected ReadyWithLabel to be called with 'spec:auth', got %q", label)
		}
	}
}

// TestRunnerLabelFiltering_MultipleLabels verifies that when multiple label filters are set,
// the runner iterates through each label and collects beads, then processes in priority order
func TestRunnerLabelFiltering_MultipleLabels(t *testing.T) {
	var output strings.Builder

	authBeads := []*bead.Bead{
		{ID: "auth-1", Title: "Auth task", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
	}
	paymentBeads := []*bead.Bead{
		{ID: "pay-1", Title: "Payment task", Priority: 0, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
		{ID: "pay-2", Title: "Another payment", Priority: 1, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
	}

	var processedBeads []string
	var readyWithLabelCalls []string
	closedBeads := make(map[string]bool)

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			t.Error("Ready() should not be called when label filters are set")
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalls = append(readyWithLabelCalls, label)
			if label == "spec:auth" {
				// Find first non-closed auth bead
				for i := 0; i < len(authBeads); i++ {
					if !closedBeads[authBeads[i].ID] {
						return authBeads[i], nil
					}
				}
				return nil, nil
			}
			if label == "spec:payments" {
				// Find first non-closed payment bead
				for i := 0; i < len(paymentBeads); i++ {
					if !closedBeads[paymentBeads[i].ID] {
						return paymentBeads[i], nil
					}
				}
				return nil, nil
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set label filters for multiple specs
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify all 3 beads were processed (1 auth + 2 payment)
	if len(processedBeads) != 3 {
		t.Errorf("Expected 3 beads to be processed (1 auth + 2 payments), got %d: %v", len(processedBeads), processedBeads)
	}

	// Verify ReadyWithLabel was called for each label
	foundAuth := false
	foundPayments := false
	for _, label := range readyWithLabelCalls {
		if label == "spec:auth" {
			foundAuth = true
		}
		if label == "spec:payments" {
			foundPayments = true
		}
	}
	if !foundAuth {
		t.Error("Expected ReadyWithLabel to be called with 'spec:auth'")
	}
	if !foundPayments {
		t.Error("Expected ReadyWithLabel to be called with 'spec:payments'")
	}

	// Verify beads were processed in priority order: pay-1 (P0), auth-1 (P1), pay-2 (P1)
	expectedOrder := []string{"pay-1", "auth-1", "pay-2"}
	if len(processedBeads) == 3 {
		for i, expected := range expectedOrder {
			if processedBeads[i] != expected {
				t.Errorf("Expected bead at position %d to be %q, got %q", i, expected, processedBeads[i])
			}
		}
	}
}

// TestRunnerLabelFiltering_EmptyLabelList verifies that an empty label list
// behaves the same as no labels (uses Ready())
func TestRunnerLabelFiltering_EmptyLabelList(t *testing.T) {
	var output strings.Builder

	testBeads := []*bead.Bead{
		{ID: "bead-1", Title: "Task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	currentBead := 0
	var processedBeads []string

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if currentBead >= len(testBeads) {
				return nil, nil
			}
			b := testBeads[currentBead]
			currentBead++
			return b, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			t.Errorf("ReadyWithLabel(%q) should not be called with empty label list", label)
			return nil, nil
		},
		CloseFn: func(id string) error {
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set empty label filter list - should behave like no filters
	r.SetLabelFilters([]string{})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify the bead was processed using Ready()
	if len(processedBeads) != 1 {
		t.Errorf("Expected 1 bead to be processed, got %d: %v", len(processedBeads), processedBeads)
	}
}

// TestRunnerLabelFiltering_PriorityOrderingAcrossLabels verifies that when multiple labels
// are specified, beads are still processed in priority order (P0 first, then P1, P2)
// regardless of which label they came from
func TestRunnerLabelFiltering_PriorityOrderingAcrossLabels(t *testing.T) {
	var output strings.Builder

	authBeads := []*bead.Bead{
		{ID: "auth-1", Title: "Auth P1", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "auth-2", Title: "Auth P0", Priority: 0, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
	}
	paymentBeads := []*bead.Bead{
		{ID: "pay-1", Title: "Payment P2", Priority: 2, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
		{ID: "pay-2", Title: "Payment P1", Priority: 1, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
	}

	var processedBeads []string
	closedBeads := make(map[string]bool)

	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:auth" {
				// Find highest priority (lowest number) non-closed auth bead
				var bestBead *bead.Bead
				for i := 0; i < len(authBeads); i++ {
					if !closedBeads[authBeads[i].ID] {
						if bestBead == nil || authBeads[i].Priority < bestBead.Priority {
							bestBead = authBeads[i]
						}
					}
				}
				return bestBead, nil
			}
			if label == "spec:payments" {
				// Find highest priority (lowest number) non-closed payment bead
				var bestBead *bead.Bead
				for i := 0; i < len(paymentBeads); i++ {
					if !closedBeads[paymentBeads[i].ID] {
						if bestBead == nil || paymentBeads[i].Priority < bestBead.Priority {
							bestBead = paymentBeads[i]
						}
					}
				}
				return bestBead, nil
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set label filters for multiple specs
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify beads were processed in priority order: auth-2 (P0), auth-1 (P1), pay-2 (P1), pay-1 (P2)
	expectedOrder := []string{"auth-2", "auth-1", "pay-2", "pay-1"}
	if len(processedBeads) != 4 {
		t.Errorf("Expected 4 beads to be processed, got %d: %v", len(processedBeads), processedBeads)
	}
	if len(processedBeads) == 4 {
		for i, expected := range expectedOrder {
			if processedBeads[i] != expected {
				t.Errorf("Expected bead at position %d to be %q, got %q (demonstrates cross-label priority ordering)", i, expected, processedBeads[i])
			}
		}
	}
}

// TestRunnerLabelFiltering_NoMatchingBeads verifies that when label filters are set
// but no beads match those labels, the runner exits gracefully without errors
func TestRunnerLabelFiltering_NoMatchingBeads(t *testing.T) {
	var output strings.Builder

	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			// No beads match the requested label
			return nil, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set label filter for a spec that doesn't exist
	r.SetLabelFilters([]string{"spec:nonexistent"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() should not fail when no beads match, got error: %v", err)
	}

	// Verify output contains "No more work available"
	outputStr := output.String()
	if !strings.Contains(outputStr, "No more work available") {
		t.Errorf("Expected 'No more work available' in output when no beads match label filter, got: %s", outputStr)
	}
}

// TestRunnerLabelFiltering_BeadWithMultipleMatchingLabels verifies that when a bead
// has multiple labels that match the filter list, it's only returned once (not duplicated)
func TestRunnerLabelFiltering_BeadWithMultipleMatchingLabels(t *testing.T) {
	var output strings.Builder

	// Create a bead with multiple spec labels
	beadsWithMultipleLabels := []*bead.Bead{
		{ID: "multi-1", Title: "Multi-label task", Priority: 1, Labels: []string{"spec:auth", "spec:payments"}, ExpectedOutputs: []string{}},
		{ID: "auth-1", Title: "Auth only", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
	}

	var processedBeads []string
	closedBeads := make(map[string]bool)

	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			// Return the first non-closed bead that has this label
			for i := 0; i < len(beadsWithMultipleLabels); i++ {
				b := beadsWithMultipleLabels[i]
				if closedBeads[b.ID] {
					continue
				}
				// Check if this bead has the requested label
				for _, l := range b.Labels {
					if l == label {
						return b, nil
					}
				}
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set label filters for both labels that multi-1 has
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify multi-1 was only processed once (not twice despite having both labels)
	multiCount := 0
	for _, id := range processedBeads {
		if id == "multi-1" {
			multiCount++
		}
	}
	if multiCount != 1 {
		t.Errorf("Expected bead 'multi-1' to be processed exactly once despite having both labels, got %d times. Processed beads: %v", multiCount, processedBeads)
	}

	// Verify total beads processed is 2 (not 3)
	if len(processedBeads) != 2 {
		t.Errorf("Expected 2 beads to be processed (multi-1 once + auth-1), got %d: %v", len(processedBeads), processedBeads)
	}
}

// TestRunnerLabelFiltering_DeduplicationInGetNextBead verifies that getNextBead()
// deduplicates beads when the same bead is returned by multiple ReadyWithLabel calls
func TestRunnerLabelFiltering_DeduplicationInGetNextBead(t *testing.T) {
	var output strings.Builder

	// This bead has both labels, so ReadyWithLabel will return it for both label queries
	multiLabelBead := &bead.Bead{
		ID:              "multi-1",
		Title:           "Bead with multiple labels",
		Priority:        1,
		Labels:          []string{"spec:auth", "spec:payments"},
		ExpectedOutputs: []string{},
	}

	var readyWithLabelCallCount int
	var processedBeads []string

	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCallCount++
			// Both label queries return the same bead (simulating bd behavior)
			if label == "spec:auth" || label == "spec:payments" {
				return multiLabelBead, nil
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			processedBeads = append(processedBeads, id)
			// After closing, subsequent calls should return nil
			multiLabelBead = nil
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	deps := Deps{
		Beads:    mockBeads,
		Claude:   mockClaude,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Set label filters for both labels
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify the bead was only processed once
	if len(processedBeads) != 1 {
		t.Errorf("Expected bead to be processed exactly once, got %d times: %v", len(processedBeads), processedBeads)
	}

	// Verify ReadyWithLabel was called for both labels (showing deduplication happened)
	if readyWithLabelCallCount < 2 {
		t.Errorf("Expected ReadyWithLabel to be called at least twice (once per label), got %d calls", readyWithLabelCallCount)
	}
}
