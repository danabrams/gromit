package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// setupDecomposeGuardTest creates a runner for depth/guard tests.
func setupDecomposeGuardTest(t *testing.T, cfg *config.Config) (*Runner, *strings.Builder) {
	t.Helper()
	return setupDecomposeGuardTestWithBeads(t, cfg, &mockBeadClient{})
}

// setupDecomposeGuardTestWithBeads creates a runner with a custom bead client.
func setupDecomposeGuardTestWithBeads(t *testing.T, cfg *config.Config, beads BeadClient) (*Runner, *strings.Builder) {
	t.Helper()
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    beads,
			Router:   newMockRouter(),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}
	return r, &buf
}

// TestDecomposeTask_UsesConfigurableMaxDepth verifies that Loop.MaxDecomposeDepth
// controls when DecomposeTask refuses to decompose further.
// A bead at depth == MaxDecomposeDepth should return an error.
func TestDecomposeTask_UsesConfigurableMaxDepth(t *testing.T) {
	cfg := &config.Config{
		Loop: config.LoopConfig{
			MaxDecomposeDepth: 3,
		},
	}
	r, _ := setupDecomposeGuardTest(t, cfg)

	// Bead ID with 3 dots = depth 3 (e.g. "gromit-abc.1.2.3")
	b := &bead.Bead{
		ID:              "gromit-abc.1.2.3",
		Title:           "Some deep task",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	_, err := r.DecomposeTask(context.Background(), b)
	if err == nil {
		t.Fatal("expected error for bead at configured max depth 3, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to decompose further") {
		t.Errorf("expected 'refusing to decompose further' in error, got: %v", err)
	}
}

// TestDecomposeTask_DefaultMaxDepthAllowsDepthNine verifies that with no
// MaxDecomposeDepth configured (0 → defaults to 10), depth 9 is allowed.
func TestDecomposeTask_DefaultMaxDepthAllowsDepthNine(t *testing.T) {
	cfg := &config.Config{
		Loop: config.LoopConfig{
			MaxDecomposeDepth: 0, // unset → should default to 10
		},
	}
	r, _ := setupDecomposeGuardTest(t, cfg)

	// Build an ID with exactly 9 dots
	id := "gromit-abc." + strings.Repeat("x.", 8) + "last"
	depth := strings.Count(id, ".")
	if depth != 9 {
		t.Fatalf("test setup error: expected ID depth 9, got %d (id=%q)", depth, id)
	}

	b := &bead.Bead{
		ID:              id,
		Title:           "Deep but allowed",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// With default max depth of 10, depth 9 should NOT return a depth error.
	// It will fail for other reasons (mock router returns no provider for decompose),
	// but the error should NOT be about refusing to decompose.
	_, err := r.DecomposeTask(context.Background(), b)
	if err != nil && strings.Contains(err.Error(), "refusing to decompose further") {
		t.Errorf("depth 9 should be allowed with default max depth 10, got refusal: %v", err)
	}
}

// TestDecomposeTask_DefaultMaxDepthBlocksDepthTen verifies that with default
// MaxDecomposeDepth (10), depth 10 is blocked.
func TestDecomposeTask_DefaultMaxDepthBlocksDepthTen(t *testing.T) {
	cfg := &config.Config{
		Loop: config.LoopConfig{
			MaxDecomposeDepth: 0, // unset → should default to 10
		},
	}
	r, _ := setupDecomposeGuardTest(t, cfg)

	// Build an ID with exactly 10 dots
	id := "gromit-abc." + strings.Repeat("x.", 9) + "last"
	depth := strings.Count(id, ".")
	if depth != 10 {
		t.Fatalf("test setup error: expected ID depth 10, got %d (id=%q)", depth, id)
	}

	b := &bead.Bead{
		ID:              id,
		Title:           "Too deep task",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	_, err := r.DecomposeTask(context.Background(), b)
	if err == nil {
		t.Fatal("expected error for bead at default max depth 10, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to decompose further") {
		t.Errorf("expected 'refusing to decompose further', got: %v", err)
	}
}

// TestDecomposeTask_LogsWarningAtDepthOverFive verifies that DecomposeTask logs
// a depth warning when depth > 5, even though decomposition is not blocked.
func TestDecomposeTask_LogsWarningAtDepthOverFive(t *testing.T) {
	cfg := &config.Config{} // defaults applied by NewRunnerWithDeps

	r, buf := setupDecomposeGuardTest(t, cfg)

	// Build an ID with exactly 6 dots (depth 6, above warning threshold of 5)
	id := "gromit-abc." + strings.Repeat("x.", 5) + "last"
	depth := strings.Count(id, ".")
	if depth != 6 {
		t.Fatalf("test setup error: expected ID depth 6, got %d (id=%q)", depth, id)
	}

	b := &bead.Bead{
		ID:              id,
		Title:           "Moderately deep",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// This will fail at the provider step (no real provider), but the
	// warning should have been logged before that.
	r.DecomposeTask(context.Background(), b) //nolint:errcheck

	logs := buf.String()
	if !strings.Contains(logs, "depth") {
		t.Errorf("expected a depth warning in logs at depth 6, got logs: %q", logs)
	}
}

// TestCreateSubBeads_SkipsSingleChild verifies that CreateSubBeads returns an error
// and creates no beads when given exactly 1 sub-task (single-child guard).
func TestCreateSubBeads_SkipsSingleChild(t *testing.T) {
	cfg := &config.Config{}

	var createdTitles []string
	mockBeads := &mockBeadClient{
		CreateWithParentAndDescriptionFn: func(title string, _ int, _ []string, _ []string, _ string, _ string) (*bead.Bead, error) {
			createdTitles = append(createdTitles, title)
			return &bead.Bead{ID: "sub-" + title, Title: title, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	r, _ := setupDecomposeGuardTestWithBeads(t, cfg, mockBeads)

	b := &bead.Bead{
		ID:              "test-abc",
		Title:           "Some task",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}
	subTasks := []SubTask{
		{Title: "Only child", Description: "The only sub-task", AcceptanceCriteria: []string{}},
	}

	err := r.CreateSubBeads(context.Background(), b, subTasks)
	if err == nil {
		t.Fatal("expected error for single-child decomposition, got nil")
	}
	if !strings.Contains(err.Error(), "single child") && !strings.Contains(err.Error(), "atomic") {
		t.Errorf("expected error about single child or atomic scope, got: %v", err)
	}
	if len(createdTitles) > 0 {
		t.Errorf("expected no beads created for single-child decomposition, got: %v", createdTitles)
	}
}
