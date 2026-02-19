package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestRunProactiveDecomposition_DecomposesKeywordBead verifies that a bead whose title
// contains a decomposition-trigger keyword (e.g., "refactor") is automatically
// decomposed before first attempt, creating sub-beads and closing the original.
func TestRunProactiveDecomposition_DecomposesKeywordBead(t *testing.T) {
	precheckDisabled := false
	autoPushDisabled := false
	cfg := &config.Config{
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	subTasksJSON, _ := json.Marshal([]runtypes.SubTask{
		{Title: "Sub-task A", Description: "Do part A", AcceptanceCriteria: []string{"A done"}},
		{Title: "Sub-task B", Description: "Do part B", AcceptanceCriteria: []string{"B done"}},
	})

	callCount := 0
	var createdSubBeads []string
	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount == 1 {
				return &bead.Bead{
					ID:              "refactor-1",
					Title:           "Refactor config loading to use interfaces",
					Priority:        2,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			createdSubBeads = append(createdSubBeads, title)
			return &bead.Bead{ID: "sub-" + title, Title: title, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	// Claude returns sub-task JSON (used by DecomposeTask to parse sub-tasks)
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: string(subTasksJSON)}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	err = r.Run(context.Background(), 1, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Sub-beads should have been created (indicating proactive decomposition occurred)
	if len(createdSubBeads) == 0 {
		t.Fatal("expected sub-beads to be created by proactive decomposition, but none were created")
	}
	if len(createdSubBeads) != 2 {
		t.Errorf("expected 2 sub-beads created, got %d: %v", len(createdSubBeads), createdSubBeads)
	}

	// The original bead should be closed (decomposed)
	found := false
	for _, id := range mockBeads.ClosedIDs {
		if id == "refactor-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected original bead refactor-1 to be closed after proactive decomposition, closed IDs: %v", mockBeads.ClosedIDs)
	}
}

// TestRunProactiveDecomposition_SkipsNonCandidateBead verifies that a bead with a
// regular title is NOT proactively decomposed (passes through to normal processing).
func TestRunProactiveDecomposition_SkipsNonCandidateBead(t *testing.T) {
	precheckDisabled := false
	autoPushDisabled := false
	cfg := &config.Config{
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	callCount := 0
	var createdSubBeads []string
	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount == 1 {
				return &bead.Bead{
					ID:              "normal-1",
					Title:           "Add retry count to iteration log",
					Priority:        2,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			createdSubBeads = append(createdSubBeads, title)
			return &bead.Bead{ID: "sub-" + title, Title: title, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouter(),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	// dry-run: normal bead goes through to build (not proactively decomposed)
	err = r.Run(context.Background(), 1, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// No sub-beads should be created (no proactive decomposition)
	if len(createdSubBeads) > 0 {
		t.Errorf("expected no sub-beads created for non-candidate bead, got: %v", createdSubBeads)
	}
}

// TestRunProactiveDecomposition_RetriesOnParseFailure verifies that when the first
// decomposition attempt returns non-JSON output, a second attempt is made. If the
// retry succeeds, sub-beads are created normally.
func TestRunProactiveDecomposition_RetriesOnParseFailure(t *testing.T) {
	precheckDisabled := false
	autoPushDisabled := false
	cfg := &config.Config{
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	subTasksJSON, _ := json.Marshal([]runtypes.SubTask{
		{Title: "Sub-task A", Description: "Do part A", AcceptanceCriteria: []string{"A done"}},
		{Title: "Sub-task B", Description: "Do part B", AcceptanceCriteria: []string{"B done"}},
	})

	claudeCallCount := 0
	var createdSubBeads []string
	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			// First call returns the refactor bead; subsequent calls return nil (no more work)
			if len(createdSubBeads) == 0 {
				return &bead.Bead{
					ID:              "refactor-retry",
					Title:           "Refactor config loading",
					Priority:        2,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			createdSubBeads = append(createdSubBeads, title)
			return &bead.Bead{ID: "sub-" + title, Title: title, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	// First call returns non-JSON; second call returns valid JSON
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			claudeCallCount++
			if claudeCallCount == 1 {
				return &claude.Result{Success: true, Output: "Here are the sub-tasks I'd recommend..."}, nil
			}
			return &claude.Result{Success: true, Output: string(subTasksJSON)}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	err = r.Run(context.Background(), 1, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Claude should have been called twice for decomposition (first failed, retry succeeded)
	if claudeCallCount < 2 {
		t.Errorf("expected at least 2 Claude calls (initial + retry), got %d", claudeCallCount)
	}

	// Sub-beads should have been created on retry (2 sub-tasks)
	if len(createdSubBeads) != 2 {
		t.Errorf("expected 2 sub-beads created after retry, got %d: %v", len(createdSubBeads), createdSubBeads)
	}
}
