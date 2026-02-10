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

	_, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// TODO: This will fail until Runner accepts label filters
	// For now, this test documents the expected behavior
	// When implementation is done, add:
	// r.SetLabelFilters([]string{"spec:auth"})

	// This will currently fail because Runner doesn't have SetLabelFilters yet
	// err = r.Run(ctx, 10, time.Time{}, false)
	t.Skip("Skipping until Runner.SetLabelFilters() is implemented")

	// After implementation:
	// - Verify only auth beads were processed
	// - Verify ReadyWithLabel was called with "spec:auth"
	// - Verify Ready() was never called
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

	authIndex := 0
	paymentIndex := 0
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
			if label == "spec:payments" && paymentIndex < len(paymentBeads) {
				b := paymentBeads[paymentIndex]
				paymentIndex++
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

	_, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// TODO: This will fail until Runner accepts label filters
	// For now, this test documents the expected behavior
	// When implementation is done, add:
	// r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	// This will currently fail because Runner doesn't have SetLabelFilters yet
	t.Skip("Skipping until Runner.SetLabelFilters() is implemented")

	// After implementation:
	// - Verify all 3 beads were processed (1 auth + 2 payment)
	// - Verify ReadyWithLabel was called for each label
	// - Verify beads were processed in priority order across labels
	// - Expected order: pay-1 (P0), auth-1 (P1), pay-2 (P1)
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

	// TODO: This will fail until Runner accepts label filters
	// When implementation is done, add:
	// r.SetLabelFilters([]string{}) // empty list

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

	authIndex := 0
	paymentIndex := 0
	var processedBeads []string

	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:auth" && authIndex < len(authBeads) {
				b := authBeads[authIndex]
				authIndex++
				return b, nil
			}
			if label == "spec:payments" && paymentIndex < len(paymentBeads) {
				b := paymentBeads[paymentIndex]
				paymentIndex++
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

	_, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// TODO: This will fail until Runner accepts label filters
	// When implementation is done, add:
	// r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	t.Skip("Skipping until Runner.SetLabelFilters() is implemented")

	// After implementation:
	// - Verify beads were processed in priority order: auth-2 (P0), auth-1 (P1), pay-2 (P1), pay-1 (P2)
	// - This demonstrates cross-label priority ordering works correctly
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

	_, err := NewRunnerWithDeps(cfg, &output, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// TODO: This will fail until Runner accepts label filters
	// When implementation is done, add:
	// r.SetLabelFilters([]string{"spec:nonexistent"})

	t.Skip("Skipping until Runner.SetLabelFilters() is implemented")

	// After implementation:
	// - Verify Run() completes without error
	// - Verify output contains "No more work available"
}
