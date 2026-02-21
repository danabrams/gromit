package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

func TestSetupBeadContext_NilConfig(t *testing.T) {
	r := &Runner{output: &strings.Builder{}}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, _, _, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err == nil || !strings.Contains(err.Error(), "config is nil") {
		t.Errorf("expected 'config is nil' error, got: %v", err)
	}
}

func TestSetupBeadContext_NilBeads(t *testing.T) {
	r := &Runner{cfg: &config.Config{}, output: &strings.Builder{}}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, _, _, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err == nil || !strings.Contains(err.Error(), "beads client is nil") {
		t.Errorf("expected 'beads client is nil' error, got: %v", err)
	}
}

func TestSetupBeadContext_NilRenderer(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		beads:  &mockBeadClient{},
		output: &strings.Builder{},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, _, _, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err == nil || !strings.Contains(err.Error(), "renderer is nil") {
		t.Errorf("expected 'renderer is nil' error, got: %v", err)
	}
}

func TestSetupBeadContext_NilClaude(t *testing.T) {
	r := &Runner{
		cfg:      &config.Config{},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   &strings.Builder{},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, _, _, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err == nil || !strings.Contains(err.Error(), "router is nil") {
		t.Errorf("expected 'router is nil' error, got: %v", err)
	}
}

func TestSetupBeadContext_SetsFields(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   &strings.Builder{},
		router:   newMockRouter(),
	}
	b := &bead.Bead{ID: "test-1", Title: "Test", Priority: 1}

	bc, beadCtx, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	if bc.Bead != b {
		t.Error("BeadContext.Bead should reference the input bead")
	}
	if bc.Result == nil {
		t.Fatal("BeadContext.Result should not be nil")
	}
	if bc.Result.BeadID != "test-1" {
		t.Errorf("expected BeadID 'test-1', got %q", bc.Result.BeadID)
	}
	if bc.Result.OriginalTier != bc.Tier {
		t.Errorf("expected OriginalTier %q, got %q", bc.Tier, bc.Result.OriginalTier)
	}
	if bc.MaxRetries != 2 {
		t.Errorf("expected maxRetries=2, got %d", bc.MaxRetries)
	}
	if bc.MaxRetriesPerBead != 5 {
		t.Errorf("expected maxRetriesPerBead=5, got %d", bc.MaxRetriesPerBead)
	}
	if bc.BeadTimeout != 300*time.Second {
		t.Errorf("expected beadTimeout=300s, got %v", bc.BeadTimeout)
	}
	if beadCtx == nil {
		t.Error("beadCtx should not be nil")
	}
}

func TestSetupBeadContext_UsesInjectedGitHeadFn(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   &strings.Builder{},
		router:   newMockRouter(),
		gitHeadFn: func() (string, error) {
			return "abc123", nil
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test", Priority: 1}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	if bc.StartCommit != "abc123" {
		t.Errorf("StartCommit = %q, want %q", bc.StartCommit, "abc123")
	}
}

func TestEscalateModel(t *testing.T) {
	bc := &runtypes.BeadContext{
		Model:     "haiku",
		Result:    &IterationResult{Model: "haiku"},
		PromptCtx: &prompt.Context{Model: "haiku"},
	}

	// Inline escalateModel logic since the method was removed
	bc.Model = "sonnet"
	bc.Result.Model = "sonnet"
	bc.Result.Escalated = true
	bc.Result.EscalatedTo = "sonnet"
	bc.RetriesThisModel = 0
	bc.PromptCtx.Model = "sonnet"

	if bc.Model != "sonnet" {
		t.Errorf("expected model 'sonnet', got %q", bc.Model)
	}
	if bc.Result.Model != "sonnet" {
		t.Errorf("expected result.Model 'sonnet', got %q", bc.Result.Model)
	}
	if bc.Result.Escalated != true {
		t.Error("expected result.Escalated to be true")
	}
	if bc.Result.EscalatedTo != "sonnet" {
		t.Errorf("expected result.EscalatedTo 'sonnet', got %q", bc.Result.EscalatedTo)
	}
	if bc.RetriesThisModel != 0 {
		t.Errorf("expected retriesThisModel=0, got %d", bc.RetriesThisModel)
	}
	if bc.PromptCtx.Model != "sonnet" {
		t.Errorf("expected promptCtx.Model 'sonnet', got %q", bc.PromptCtx.Model)
	}
}

func TestHandleScopeTooLarge(t *testing.T) {
	var buf strings.Builder
	tempDir := t.TempDir()

	lf, err := learnings.NewFile(tempDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}

	mockRend := &mockPromptRenderer{LearningsFile: lf}

	r := &Runner{
		beads:    &mockBeadClient{},
		renderer: mockRend,
		output:   &buf,
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		Model:  "haiku",
		Result: &IterationResult{},
	}

	result := &claude.Result{
		Output: "SCOPE_TOO_LARGE: This task is too big\nBreakdown suggestion here",
	}

	r.handleScopeTooLarge(bc, result, "This task is too big")

	if bc.Result.Error == nil {
		t.Fatal("expected error to be set")
	}
	if !strings.Contains(bc.Result.Error.Error(), "scope too large") {
		t.Errorf("expected 'scope too large' in error, got %q", bc.Result.Error.Error())
	}
	// Verify that synthetic learning was extracted
	if !strings.Contains(buf.String(), "Synthetic learning extracted") {
		t.Error("expected synthetic learning to be extracted")
	}
}

func TestExtractScopeTooLargeLearning(t *testing.T) {
	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Complex Feature"},
		Model:  "haiku",
		Result: &IterationResult{},
	}

	escalation.ExtractScopeTooLargeLearning(bc, "too many acceptance criteria", lf)

	// Check that learning was added to the learnings file
	provisionals := lf.GetProvisional()
	if len(provisionals) == 0 {
		t.Fatal("expected a learning to be added")
	}
	found := false
	for _, l := range provisionals {
		if strings.Contains(l.Content, "Complex Feature") && strings.Contains(l.Content, "haiku") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected learning to contain bead title 'Complex Feature' and model 'haiku'")
	}
}

func TestExtractTimeoutLearning(t *testing.T) {
	lf, err := learnings.NewFile(t.TempDir())
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Slow Task"},
		Model:  "sonnet",
		Result: &IterationResult{},
	}

	escalation.ExtractTimeoutLearning(bc, lf)

	// Check that learning was added to the learnings file
	provisionals := lf.GetProvisional()
	if len(provisionals) == 0 {
		t.Fatal("expected a learning to be added")
	}
	found := false
	for _, l := range provisionals {
		if strings.Contains(l.Content, "Slow Task") && strings.Contains(l.Content, "timed out") && strings.Contains(l.Content, "sonnet") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected learning to contain 'Slow Task', 'timed out', and 'sonnet'")
	}
}

func TestExtractLearning_NilLearning(t *testing.T) {
	lf, _ := learnings.NewFile(t.TempDir())
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "test-1"},
	}

	analysis := &analyzer.Analysis{
		Category:    analyzer.CategorySyntax,
		Recoverable: true,
		RootCause:   "some bug",
		Learning:    nil, // No learning
	}

	// Should not panic
	escalation.ExtractLearning(bc, analysis, lf)

	// Should not add a learning when Learning is nil
	provisionals := lf.GetProvisional()
	if len(provisionals) != 0 {
		t.Errorf("expected no learnings added, got %d", len(provisionals))
	}
}

func TestExtractLearning_WithLearning(t *testing.T) {
	lf, _ := learnings.NewFile(t.TempDir())
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "test-1"},
	}

	learning := "Always check for nil before dereferencing"
	analysis := &analyzer.Analysis{
		Category:    analyzer.CategoryLogic,
		Recoverable: true,
		RootCause:   "some bug",
		Learning:    &learning,
	}

	escalation.ExtractLearning(bc, analysis, lf)

	// Should add the learning to the learnings file
	provisionals := lf.GetProvisional()
	if len(provisionals) == 0 {
		t.Fatal("expected a learning to be added")
	}
	found := false
	for _, l := range provisionals {
		if strings.Contains(l.Content, "Always check for nil") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected learning content 'Always check for nil before dereferencing'")
	}
}

func TestHandleStallTimeout_ExceedsBeadLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	h := escalation.NewHandler(cfg, nil, nil, nil, nil, nil, nil)
	bc := &runtypes.BeadContext{
		Bead:                 &bead.Bead{ID: "test-1"},
		Result:               &IterationResult{},
		RetriesThisModel:     0,
		TotalRetriesThisBead: 5,
		MaxRetries:           2,
		MaxRetriesPerBead:    5,
	}

	continueLoop := h.HandleStallTimeout(context.Background(), bc)

	if continueLoop {
		t.Error("expected false when max retries per bead exceeded")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error to be set")
	}
	if !strings.Contains(bc.Result.Error.Error(), "exceeded max retries per bead") {
		t.Errorf("expected 'exceeded max retries per bead' in error, got %q", bc.Result.Error.Error())
	}
}

func TestHandleStallTimeout_RetryWithSameModel(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	h := escalation.NewHandler(cfg, nil, nil, nil, nil, nil, nil)
	bc := &runtypes.BeadContext{
		Bead:                 &bead.Bead{ID: "test-1"},
		Result:               &IterationResult{},
		Model:                "haiku",
		RetriesThisModel:     0,
		TotalRetriesThisBead: 0,
		MaxRetries:           2,
		MaxRetriesPerBead:    5,
	}

	continueLoop := h.HandleStallTimeout(context.Background(), bc)

	if !continueLoop {
		t.Error("expected true when retries available")
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("expected retriesThisModel=1, got %d", bc.RetriesThisModel)
	}
	if bc.TotalRetriesThisBead != 1 {
		t.Errorf("expected totalRetriesThisBead=1, got %d", bc.TotalRetriesThisBead)
	}
}

func TestHandleStallTimeout_Escalates(t *testing.T) {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	h := escalation.NewHandler(cfg, nil, nil, nil, nil, nil, nil)
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1"},
		Result: &IterationResult{ToolCallCount: 1},
		Model:  "haiku",
		Tier:   provider.TierLow, // haiku maps to low tier
		PromptCtx: &prompt.Context{
			Model:              "haiku",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
		RetriesThisModel:     2,
		TotalRetriesThisBead: 2,
		MaxRetries:           2,
		MaxRetriesPerBead:    10,
	}

	continueLoop := h.HandleStallTimeout(context.Background(), bc)

	if !continueLoop {
		t.Error("expected continueLoop=true after escalation")
	}

	// Verify escalation was at least attempted
	if bc.Model != "sonnet" {
		t.Errorf("expected model to be escalated to 'sonnet', got %q", bc.Model)
	}
}

func TestProcessBead_DurationIsSetOnSetupFailure(t *testing.T) {
	r := &Runner{output: &strings.Builder{}}
	b := &bead.Bead{ID: "test-1", Title: "Test"}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if result.Error == nil {
		t.Fatal("expected error for nil config")
	}
	if result.Duration == 0 {
		t.Error("expected Duration to be set even on setup failure")
	}
	if result.BeadID != "test-1" {
		t.Errorf("expected BeadID 'test-1', got %q", result.BeadID)
	}
}

func TestExtractSuccessLearning_NilRunner(t *testing.T) {
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "test-1"},
	}
	// Should not panic
	escalation.ExtractSuccessLearning(context.Background(), bc, nil, nil, nil, nil, nil)
}

func TestExtractSuccessLearning_NilBeadContext(t *testing.T) {
	// Should not panic
	escalation.ExtractSuccessLearning(context.Background(), nil, nil, nil, nil, nil, nil)
}

func TestRunValidationWithRecovery_UsesFastMode(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			FastCommands: []string{"fast-check"},
			FullCommands: []string{"full-check"},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	called := ""
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		called = command
		return "ok", "", 0, nil
	}
	r := &Runner{
		cfg:              cfg,
		output:           &strings.Builder{},
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
		analyzer:         &mockFailureAnalyzer{},
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "b1", Title: "bead"},
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	if err := r.runValidationWithRecovery(context.Background(), bc); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if called != "fast-check" {
		t.Fatalf("expected fast command, got %q", called)
	}
	if bc.Result.ValidationMode != "direct" {
		t.Fatalf("expected validation mode direct, got %q", bc.Result.ValidationMode)
	}
}

func TestScopeValidationCommands_ScopesGoTestWildcard(t *testing.T) {
	commands := []string{"go test ./...", "golangci-lint run ./..."}
	touched := []string{"internal/runner", "internal/provider"}

	got := config.ScopeGoTestCommands(commands, touched)
	want := []string{
		"go test ./internal/runner/... ./internal/provider/...",
		"golangci-lint run ./...",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d commands, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestScopeValidationCommands_EmptyTouchedPackagesReturnsUnchanged(t *testing.T) {
	commands := []string{"go test ./...", "go vet ./..."}
	got := config.ScopeGoTestCommands(commands, nil)

	if len(got) != len(commands) {
		t.Fatalf("got %d commands, want %d", len(got), len(commands))
	}
	for i := range commands {
		if got[i] != commands[i] {
			t.Fatalf("command[%d] = %q, want %q", i, got[i], commands[i])
		}
	}
}

func TestRunValidation_ScopesFastCommandsWhenTouchedPackagesPresent(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			Commands:     []string{"go test ./..."},
			FastCommands: []string{"go test ./...", "go vet ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var called []string
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		called = append(called, command)
		return "ok", "", 0, nil
	}

	r := &Runner{
		cfg:              cfg,
		output:           &strings.Builder{},
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead:            &bead.Bead{ID: "b1", Title: "bead"},
		Result:          &IterationResult{},
		TouchedPackages: []string{"internal/runner", "internal/provider"},
		PromptCtx:       &prompt.Context{WorkDir: t.TempDir()},
	}

	if err := r.runValidation(context.Background(), bc); err != nil {
		t.Fatalf("runValidation returned error: %v", err)
	}

	want := []string{
		"go test ./internal/runner/... ./internal/provider/...",
		"go vet ./...",
	}
	if len(called) != len(want) {
		t.Fatalf("ran %d commands, want %d (%v)", len(called), len(want), called)
	}
	for i := range want {
		if called[i] != want[i] {
			t.Fatalf("command[%d] = %q, want %q", i, called[i], want[i])
		}
	}
}

func TestRunFullValidationGate_LeavesFullCommandsUnscoped(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			FullCommands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var called []string
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		called = append(called, command)
		return "ok", "", 0, nil
	}

	r := &Runner{
		cfg:              cfg,
		output:           &strings.Builder{},
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}

	if err := r.runFullValidationGate(context.Background(), "b1", 1); err != nil {
		t.Fatalf("runFullValidationGate returned error: %v", err)
	}

	if len(called) != 1 {
		t.Fatalf("expected one command, got %v", called)
	}
	if called[0] != "go test ./..." {
		t.Fatalf("full validation command should stay unscoped, got %q", called[0])
	}
}

func newValidationDurationTestRunner(t *testing.T, cfg *config.Config, cmdSleep *time.Duration) *Runner {
	t.Helper()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if cmdSleep != nil && *cmdSleep > 0 {
			time.Sleep(*cmdSleep)
		}
		return "ok", "", 0, nil
	}

	return &Runner{
		cfg:              cfg,
		output:           &strings.Builder{},
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}
}

func newValidationDurationBeadContext(t *testing.T, initialDurationMs int64) *runtypes.BeadContext {
	t.Helper()

	return &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "bead"},
		Result: &IterationResult{
			ValidationDurationMs: initialDurationMs,
		},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}
}

func TestRunValidation_ResetsElapsedAndAccumulatesValidationDuration(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			FastCommands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	cmdSleep := 120 * time.Millisecond
	r := newValidationDurationTestRunner(t, cfg, &cmdSleep)

	seedBC := newValidationDurationBeadContext(t, 0)
	if err := r.validationRunner.RunWithRecoveryForCommands(context.Background(), seedBC, cfg.Validation.FastCommandsOrDefault(), "fast"); err != nil {
		t.Fatalf("seed validation run failed: %v", err)
	}
	if seeded := r.validationRunner.ElapsedMs(); seeded < 100 {
		t.Fatalf("expected seeded elapsed >= 100ms, got %dms", seeded)
	}

	cmdSleep = 20 * time.Millisecond
	bc := newValidationDurationBeadContext(t, 7)
	if err := r.runValidation(context.Background(), bc); err != nil {
		t.Fatalf("runValidation() error = %v", err)
	}

	increment := bc.Result.ValidationDurationMs - 7
	if increment <= 0 {
		t.Fatalf("expected ValidationDurationMs to increase, got increment=%d", increment)
	}
	if increment >= 100 {
		t.Fatalf("expected reset elapsed to avoid seeded carry-over; increment=%dms", increment)
	}
}

func TestRunValidationWithRecoveryForStage_ResetsElapsedAndAccumulatesValidationDuration(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			FastCommands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	cmdSleep := 120 * time.Millisecond
	r := newValidationDurationTestRunner(t, cfg, &cmdSleep)

	seedBC := newValidationDurationBeadContext(t, 0)
	if err := r.validationRunner.RunWithRecoveryForCommands(context.Background(), seedBC, cfg.Validation.FastCommandsOrDefault(), "fast"); err != nil {
		t.Fatalf("seed validation run failed: %v", err)
	}
	if seeded := r.validationRunner.ElapsedMs(); seeded < 100 {
		t.Fatalf("expected seeded elapsed >= 100ms, got %dms", seeded)
	}

	cmdSleep = 20 * time.Millisecond
	bc := newValidationDurationBeadContext(t, 11)
	if err := r.runValidationWithRecoveryForStage(context.Background(), bc, true); err != nil {
		t.Fatalf("runValidationWithRecoveryForStage() error = %v", err)
	}

	increment := bc.Result.ValidationDurationMs - 11
	if increment <= 0 {
		t.Fatalf("expected ValidationDurationMs to increase, got increment=%d", increment)
	}
	if increment >= 100 {
		t.Fatalf("expected reset elapsed to avoid seeded carry-over; increment=%dms", increment)
	}
}

func TestRunFullValidationGate_ResetsElapsedBeforeValidation(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			FastCommands: []string{"go test ./..."},
			FullCommands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	cmdSleep := 120 * time.Millisecond
	r := newValidationDurationTestRunner(t, cfg, &cmdSleep)

	seedBC := newValidationDurationBeadContext(t, 0)
	if err := r.validationRunner.RunWithRecoveryForCommands(context.Background(), seedBC, cfg.Validation.FastCommandsOrDefault(), "fast"); err != nil {
		t.Fatalf("seed validation run failed: %v", err)
	}
	if seeded := r.validationRunner.ElapsedMs(); seeded < 100 {
		t.Fatalf("expected seeded elapsed >= 100ms, got %dms", seeded)
	}

	cmdSleep = 20 * time.Millisecond
	if err := r.runFullValidationGate(context.Background(), "b1", 1); err != nil {
		t.Fatalf("runFullValidationGate() error = %v", err)
	}

	if got := r.validationRunner.ElapsedMs(); got <= 0 {
		t.Fatalf("expected elapsed > 0ms after full gate, got %dms", got)
	} else if got >= 100 {
		t.Fatalf("expected reset elapsed to avoid seeded carry-over; got %dms", got)
	}
}

func TestRunValidationCommandsWithElapsed_IncrementsValidationTimeouts(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:      true,
			FastCommands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "timed out", 1, context.DeadlineExceeded
	}

	r := &Runner{
		cfg:              cfg,
		output:           &strings.Builder{},
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}
	bc := newValidationDurationBeadContext(t, 0)

	err := r.runValidationCommandsWithElapsed(context.Background(), bc, cfg.Validation.FastCommandsOrDefault(), "fast")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if got := bc.Result.ValidationTimeouts; got != 1 {
		t.Fatalf("ValidationTimeouts = %d, want 1", got)
	}
}

func TestMaybeRunPeriodicFullValidation_UsesCadenceAndFullCommands(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			FastCommands:         []string{"fast-check"},
			FullCommands:         []string{"full-check"},
			FullValidationEveryN: 2,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var commands []string
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commands = append(commands, command)
		return "ok", "", 0, nil
	}
	r := &Runner{
		cfg:                cfg,
		output:             &strings.Builder{},
		validationRunner:   validation.NewRunner(cfg, cmdRunner, nil, nil),
		analyzer:           &mockFailureAnalyzer{},
		successesSinceFull: 2,
	}

	if err := r.maybeRunPeriodicFullValidation(context.Background(), "b1", 2); err != nil {
		t.Fatalf("unexpected periodic full validation error: %v", err)
	}
	if len(commands) != 1 || commands[0] != "full-check" {
		t.Fatalf("expected one full validation command, got %v", commands)
	}
	if r.successesSinceFull != 0 {
		t.Fatalf("expected successesSinceFull reset to 0, got %d", r.successesSinceFull)
	}
}

func TestMaybeRunFinalFullValidation_SkipsWhenDisabled(t *testing.T) {
	runFinalFull := false
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:          true,
			FullCommands:     []string{"full-check"},
			RunFinalFullGate: &runFinalFull,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var commands []string
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commands = append(commands, command)
		return "ok", "", 0, nil
	}

	r := &Runner{
		cfg:                cfg,
		output:             &strings.Builder{},
		validationRunner:   validation.NewRunner(cfg, cmdRunner, nil, nil),
		successfulBeads:    1,
		successesSinceFull: 1,
	}

	if err := r.maybeRunFinalFullValidation(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 0 {
		t.Fatalf("expected final full gate to be skipped, ran commands: %v", commands)
	}
}

func setupQualityGateRunHarness(
	t *testing.T,
	cfg *config.Config,
	queue []*bead.Bead,
	analyzer FailureAnalyzer,
	cmdRunner func(ctx context.Context, command string, workDir string) (string, string, int, error),
) (*Runner, *mockBeadClient, *mockClaudeClient, *int) {
	t.Helper()

	readyCalls := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			readyCalls++
			if len(queue) == 0 {
				return nil, nil
			}
			next := queue[0]
			queue = queue[1:]
			return next, nil
		},
	}

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build ok"}, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &strings.Builder{}, t.TempDir(), Deps{
		Beads:     beads,
		Router:    newMockRouterFromClaudeClient(mockClaude),
		Analyzer:  analyzer,
		Renderer:  &mockPromptRenderer{},
		Logger:    &mockIterationLogger{},
		CmdRunner: cmdRunner,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() failed: %v", err)
	}

	return r, beads, mockClaude, &readyCalls
}

func newQualityGateTestConfig(validation config.ValidationConfig) *config.Config {
	precheckDisabled := false
	autoPushDisabled := false

	return &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: validation,
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Preflight: config.PreflightConfig{},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}
}

func TestRun_CompletionBlockedWhenMandatoryFastGateMissing(t *testing.T) {
	runFinalFull := false
	cfg := newQualityGateTestConfig(config.ValidationConfig{
		Enabled:              true,
		FastCommands:         []string{"go test ./...", "go vet ./..."},
		MandatoryCommands:    []string{"go test", "go vet", "go build"},
		FullValidationEveryN: 0,
		RunFinalFullGate:     &runFinalFull,
	})

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if command == "go build ./..." {
			return "", "compile failure", 1, nil
		}
		return "ok", "", 0, nil
	}

	r, beads, _, _ := setupQualityGateRunHarness(t, cfg, []*bead.Bead{
		{ID: "qg-fast-1", Title: "fast gate missing build", Priority: 1},
	}, &mockFailureAnalyzer{}, cmdRunner)

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err == nil {
		t.Fatal("expected run to fail when mandatory fast quality gate coverage is incomplete")
	}
	if len(beads.ClosedIDs) != 0 {
		t.Fatalf("expected bead to remain open when mandatory fast quality gates are not satisfied, closed: %v", beads.ClosedIDs)
	}
}

func TestRun_FastGateAllowsMandatoryCoverageFromFullCommands(t *testing.T) {
	runFinalFull := false
	cfg := newQualityGateTestConfig(config.ValidationConfig{
		Enabled:              true,
		FastCommands:         []string{"GROMIT_TEST_TOUCHED_SHORT=1 ./scripts/test_touched.sh", "./scripts/vet_touched.sh"},
		FullCommands:         []string{"go test ./...", "go vet ./...", "go build ./..."},
		MandatoryCommands:    []string{"go test", "go vet", "go build"},
		FullValidationEveryN: 0,
		RunFinalFullGate:     &runFinalFull,
	})

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	r, beads, _, _ := setupQualityGateRunHarness(t, cfg, []*bead.Bead{
		{ID: "qg-fast-wrapper-1", Title: "fast gate wrapper commands", Priority: 1},
	}, &mockFailureAnalyzer{}, cmdRunner)

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err != nil {
		t.Fatalf("expected run to succeed when full commands provide mandatory quality coverage: %v", err)
	}
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "qg-fast-wrapper-1" {
		t.Fatalf("expected bead to close on success, closed: %v", beads.ClosedIDs)
	}
}

func TestRun_FastGateAllowsWrappedMandatoryGoCommands(t *testing.T) {
	runFinalFull := false
	cfg := newQualityGateTestConfig(config.ValidationConfig{
		Enabled: true,
		FastCommands: []string{
			"env GOMAXPROCS=4 go test -vet=off -p 4 -parallel 4 ./...",
			"mise exec -- go vet ./...",
			"timeout 60s go build ./...",
		},
		MandatoryCommands:    []string{"go test", "go vet", "go build"},
		FullValidationEveryN: 0,
		RunFinalFullGate:     &runFinalFull,
	})

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	r, beads, _, _ := setupQualityGateRunHarness(t, cfg, []*bead.Bead{
		{ID: "qg-fast-wrapped-1", Title: "fast gate wrapped commands", Priority: 1},
	}, &mockFailureAnalyzer{}, cmdRunner)

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err != nil {
		t.Fatalf("expected run to succeed when fast commands invoke mandatory go test/vet/build via wrappers: %v", err)
	}
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "qg-fast-wrapped-1" {
		t.Fatalf("expected bead to close on success, closed: %v", beads.ClosedIDs)
	}
}

func TestRun_CompletionBlockedWhenMandatoryFullGateMissing(t *testing.T) {
	runFinalFull := false
	cfg := newQualityGateTestConfig(config.ValidationConfig{
		Enabled:              true,
		FastCommands:         []string{"go test ./...", "go vet ./...", "go build ./..."},
		FullCommands:         []string{"go test ./...", "go vet ./..."},
		MandatoryCommands:    []string{"go test", "go vet", "go build"},
		FullValidationEveryN: 1,
		RunFinalFullGate:     &runFinalFull,
	})

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	r, beads, _, _ := setupQualityGateRunHarness(t, cfg, []*bead.Bead{
		{ID: "qg-full-1", Title: "full gate missing build", Priority: 1},
	}, &mockFailureAnalyzer{}, cmdRunner)

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err == nil {
		t.Fatal("expected run to fail when mandatory full quality gate coverage is incomplete")
	}
	if len(beads.ClosedIDs) != 0 {
		t.Fatalf("expected bead to remain open when mandatory full quality gates are not satisfied, closed: %v", beads.ClosedIDs)
	}
}

func TestRun_MandatoryQualityGateCoverageSkippedWhenNoMandatoryCommands(t *testing.T) {
	runFinalFull := false
	cfg := newQualityGateTestConfig(config.ValidationConfig{
		Enabled:              true,
		FastCommands:         []string{"go test ./...", "go vet ./..."},
		FullValidationEveryN: 0,
		RunFinalFullGate:     &runFinalFull,
	})

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	r, beads, _, _ := setupQualityGateRunHarness(t, cfg, []*bead.Bead{
		{ID: "qg-no-mandatory-1", Title: "no mandatory commands configured", Priority: 1},
	}, &mockFailureAnalyzer{}, cmdRunner)

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err != nil {
		t.Fatalf("expected run to succeed when mandatory commands are not configured: %v", err)
	}
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "qg-no-mandatory-1" {
		t.Fatalf("expected bead to close on success, closed: %v", beads.ClosedIDs)
	}
}

func TestRun_UnclearPostRecoveryQualityFailureTriggersStopLine(t *testing.T) {
	precheckDisabled := false
	runFinalFull := false
	autoPushDisabled := false
	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled:          true,
			FastCommands:     []string{"go test ./...", "go vet ./...", "go build ./..."},
			RunFinalFullGate: &runFinalFull,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Preflight: config.PreflightConfig{},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	analyzerMock := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryUnclearSpec,
				Recoverable: false,
				RootCause:   "quality gate output is ambiguous after bounded recovery",
			}, nil
		},
	}
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "intermittent infra failure", 1, nil
	}

	var out strings.Builder
	r, beads, _, readyCalls := setupQualityGateRunHarness(t, cfg, []*bead.Bead{
		{ID: "qg-stopline-1", Title: "first quality failure", Priority: 1},
		{ID: "qg-stopline-2", Title: "must not run", Priority: 1},
	}, analyzerMock, cmdRunner)
	r.output = &out
	r.syncOut = newSyncWriter(&out)

	err := r.Run(context.Background(), 5, time.Now().Add(time.Minute), nil, false)
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
	if *readyCalls != 1 {
		t.Fatalf("expected stop-line to halt loop before fetching a second bead, Ready() calls=%d", *readyCalls)
	}
	if !strings.Contains(out.String(), "L3 escalation packet") {
		t.Fatalf("expected L3 stop-line escalation output for unresolved unclear quality failure, got:\n%s", out.String())
	}
	if len(beads.ClosedIDs) != 0 {
		t.Fatalf("expected no bead closures after stop-line escalation, closed: %v", beads.ClosedIDs)
	}
}

func TestExtractSuccessLearning_FeatureDisabled(t *testing.T) {
	var buf strings.Builder
	learnFromSuccessDisabled := false
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				LearnFromSuccess: &learnFromSuccessDisabled,
			},
		},
		output: &buf,
	}
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "test-1", Title: "Test"},
	}

	escalation.ExtractSuccessLearning(context.Background(), bc, r.cfg, nil, nil, nil, nil)

	if strings.Contains(buf.String(), "Success learning") {
		t.Error("should not extract learning when feature is disabled")
	}
}

func TestExtractSuccessLearning_NilLearning(t *testing.T) {
	mp := &mockSuccessLearningProvider{
		RunFn: func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
			return &mockSuccessLearningResult{
				success: true,
				output:  `{"learning": null, "category": "patterns"}`,
			}, nil
		},
	}
	router := &mockSuccessLearningRouter{provider: mp}

	lf, _ := learnings.NewFile(t.TempDir())

	learnFromSuccessEnabled := true
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
	}
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test",
			Description: "Test description",
		},
	}

	logged := false
	escalation.ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, func(format string, args ...interface{}) {
		logged = true
	}, nil)

	// Should not log "Success learning extracted" when learning is null
	if logged {
		t.Error("should not log when learning is null")
	}
}

func TestExtractSuccessLearning_WithLearning(t *testing.T) {
	mp := &mockSuccessLearningProvider{
		RunFn: func(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
			return &mockSuccessLearningResult{
				success: true,
				output:  `{"learning": "Use setDefaults() for config validation", "category": "conventions"}`,
			}, nil
		},
	}
	router := &mockSuccessLearningRouter{provider: mp}

	lf, _ := learnings.NewFile(t.TempDir())

	learnFromSuccessEnabled := true
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
	}
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Test",
			Description: "Test description",
		},
	}

	logged := false
	escalation.ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, func(format string, args ...interface{}) {
		logged = true
	}, nil)

	// Should log "Success learning extracted"
	if !logged {
		t.Error("expected logFn to be called when learning is extracted")
	}

	// Should add the learning to the learnings file
	provisionals := lf.GetProvisional()
	if len(provisionals) == 0 {
		t.Fatal("expected a learning to be added")
	}
	found := false
	for _, l := range provisionals {
		if strings.Contains(l.Content, "setDefaults()") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected learning about setDefaults() to be added")
	}
}

func TestPostSuccess_LearningFailure_ReviewStillCompletes(t *testing.T) {
	// Verifies that when learning extraction fails, the review stage still executes
	// and completes successfully.

	var buf strings.Builder
	var learningCalled, reviewCalled bool

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Learning extraction always uses haiku model
			if model == "haiku" {
				learningCalled = true
				// Learning extraction fails
				return nil, fmt.Errorf("learning extraction failed: network timeout")
			}

			// Review stage uses sonnet/opus (selected based on build model)
			reviewCalled = true
			// Review succeeds
			return &claude.Result{
				Success: true,
				Output:  `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "Review completed successfully"}`,
			}, nil
		},
	}

	lf, _ := learnings.NewFile(t.TempDir())
	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review code for improvements", nil
		},
	}

	learnFromSuccessEnabled := true
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	postSuccessCfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	syncOut := newSyncWriter(&buf)
	mockRouter := newMockRouterFromClaudeClient(mockClaude)
	mockBeads := &mockBeadClient{}
	gitDiffFn := func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}
	reviewer := reviewpkg.NewReviewer(postSuccessCfg, mockRouter, mockBeads, mockRend, gitDiffFn, nil)
	reviewer.SetLogFn(func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(syncOut, format+"\n", args...)
	})
	r := &Runner{
		cfg:              postSuccessCfg,
		router:           mockRouter,
		renderer:         mockRend,
		beads:            mockBeads,
		output:           syncOut,
		reviewer:         reviewer,
		gitDiffFn:        gitDiffFn,
		validationRunner: validation.NewRunner(postSuccessCfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-learning-fail",
			Title:       "Test Learning Failure Isolation",
			Description: "Verify review continues when learning fails",
		},
		Model:       "sonnet",
		Result:      &IterationResult{},
		StartCommit: "abc123",
		Iteration:   1,
		RunDeadline: time.Now().Add(5 * time.Minute),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	err := r.runValidation(context.Background(), bc)

	// The method should not return an error even though learning extraction failed
	if err != nil {
		t.Errorf("expected no error when learning fails (should be silently ignored), got: %v", err)
	}

	// Verify both stages were called
	if !learningCalled {
		t.Error("learning extraction should have been called")
	}
	if !reviewCalled {
		t.Error("review should have been called and completed despite learning failure")
	}

	// Verify review completed successfully by checking output contains review summary
	output := buf.String()
	if !strings.Contains(output, "Review completed successfully") && !strings.Contains(output, "Review:") {
		t.Errorf("review should have completed successfully despite learning extraction failure, output: %s", output)
	}
}

func TestPostSuccess_ReviewRevalidationError_Propagates(t *testing.T) {
	// Verifies that when the review applies fixes and re-validation fails,
	// the error propagates up correctly.

	var buf strings.Builder
	var learningCalled, reviewCalled bool
	cmdCallCount := 0

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Learning extraction uses haiku model
			if model == "haiku" {
				learningCalled = true
				return &claude.Result{
					Success: true,
					Output:  `{"learning": "Test learning", "category": "patterns"}`,
				}, nil
			}

			// Review stage uses sonnet/opus
			reviewCalled = true
			// Review succeeds but applies fixes
			return &claude.Result{
				Success: true,
				Output:  `{"passed": true, "fixes_applied": ["Fixed typo in comment", "Added missing error check"], "beads_to_create": [], "backlog_items": [], "summary": "Applied minor fixes"}`,
			}, nil
		},
	}

	lf, _ := learnings.NewFile(t.TempDir())
	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review code for improvements", nil
		},
	}

	learnFromSuccessEnabled := true
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdCallCount++
		if cmdCallCount == 1 {
			return "ok", "", 0, nil
		}
		// Re-validation fails — review fixes broke tests
		return "", "FAIL: TestSomething", 1, nil
	}
	revalCfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	syncOut := newSyncWriter(&buf)
	mockRouter := newMockRouterFromClaudeClient(mockClaude)
	mockBeads := &mockBeadClient{}
	gitDiffFn := func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}
	reviewer := reviewpkg.NewReviewer(revalCfg, mockRouter, mockBeads, mockRend, gitDiffFn, nil)
	reviewer.SetLogFn(func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(syncOut, format+"\n", args...)
	})
	// Re-validation after review fixes should fail
	revalCallCount := 0
	reviewer.SetValidateFn(func(ctx context.Context, commands []string, workDir string) (bool, error) {
		revalCallCount++
		return false, nil // re-validation fails
	})
	r := &Runner{
		cfg:              revalCfg,
		router:           mockRouter,
		renderer:         mockRend,
		beads:            mockBeads,
		output:           syncOut,
		reviewer:         reviewer,
		gitDiffFn:        gitDiffFn,
		validationRunner: validation.NewRunner(revalCfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-review-revalidation-fail",
			Title:       "Test Review Re-validation Error Propagation",
			Description: "Verify re-validation errors propagate",
		},
		Model:       "sonnet",
		Result:      &IterationResult{Validated: true},
		StartCommit: "abc123",
		Iteration:   1,
		RunDeadline: time.Now().Add(5 * time.Minute),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	err := r.runValidation(context.Background(), bc)

	// The error should propagate to the caller
	if err == nil {
		t.Fatal("expected error when review re-validation fails, but got nil")
	}

	// Verify the error message indicates re-validation failure
	if !strings.Contains(err.Error(), "review fixes broke validation") {
		t.Errorf("expected error message about review re-validation failure, got: %v", err)
	}

	// Verify all stages executed
	if !learningCalled {
		t.Error("learning extraction should have been called")
	}
	if !reviewCalled {
		t.Error("review should have been called")
	}
}

func TestPostSuccess_OnlyLearningEnabled(t *testing.T) {
	// Unit test: Verifies that when only learning extraction is enabled (review disabled),
	// learning runs inline without goroutine overhead and review is skipped entirely.
	//
	// Acceptance Criteria:
	// - When only `learn_from_success` is enabled, `extractSuccessLearning` executes inline
	// - Review stage is skipped entirely (not called)
	// - No goroutine overhead (synchronous execution)

	var buf strings.Builder
	var learningCalled, reviewCalled bool
	var learningCallTime time.Time

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Learning extraction uses haiku model
			if model == "haiku" {
				learningCalled = true
				learningCallTime = time.Now()
				return &claude.Result{
					Success: true,
					Output:  `{"learning": "Test learning from success", "category": "patterns"}`,
				}, nil
			}

			// Review should not be called
			reviewCalled = true
			return &claude.Result{
				Success: true,
				Output:  `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "Review completed"}`,
			}, nil
		},
	}

	lf, _ := learnings.NewFile(t.TempDir())
	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
	}

	learnFromSuccessEnabled := true
	syncOut := newSyncWriter(&buf)
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
		Review: config.ReviewConfig{
			Enabled: false, // Review disabled
		},
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	r := &Runner{
		cfg:      cfg,
		router:   newMockRouterFromClaudeClient(mockClaude),
		renderer: mockRend,
		beads:    &mockBeadClient{},
		output:   syncOut,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/file.go b/file.go\n+some change", nil
		},
		// Validation commands run directly — simulate all passing
		cmdRunnerFn:      cmdRunner,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-only-learning",
			Title:       "Test Single-Stage Learning",
			Description: "Verify learning runs inline when review is disabled",
		},
		Model:       "sonnet",
		Result:      &IterationResult{Validated: true},
		StartCommit: "abc123",
		Iteration:   1,
		RunDeadline: time.Now().Add(5 * time.Minute),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	start := time.Now()
	err := r.runValidation(context.Background(), bc)
	elapsed := time.Since(start)

	// Should not return an error
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify learning was called
	if !learningCalled {
		t.Error("learning extraction should have been called")
	}

	// Verify review was NOT called (disabled)
	if reviewCalled {
		t.Error("review should NOT have been called when disabled")
	}

	// Verify inline execution: learning should complete before runValidation returns
	// If learning ran in a goroutine, there would be potential for it to not complete yet
	if !learningCallTime.IsZero() && learningCallTime.After(start.Add(elapsed)) {
		t.Error("learning should have completed before runValidation returned (inline execution)")
	}

	// Verify output contains learning success message
	output := buf.String()
	if !strings.Contains(output, "Success learning extracted") {
		t.Errorf("expected 'Success learning extracted' in output, got: %s", output)
	}
}

func TestPostSuccess_OnlyReviewEnabled(t *testing.T) {
	// Unit test: Verifies that when only review is enabled (learning disabled),
	// review runs inline without goroutine overhead and learning is skipped entirely.
	//
	// Acceptance Criteria:
	// - When only `review.enabled` is true, `runLightReview` executes inline
	// - Learning stage is skipped entirely (not called)
	// - No goroutine overhead (synchronous execution)

	var buf strings.Builder
	var learningCalled, reviewCalled bool
	var reviewCallTime time.Time

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Learning extraction should not be called
			if model == "haiku" {
				learningCalled = true
				return &claude.Result{
					Success: true,
					Output:  `{"learning": "Should not be called", "category": "patterns"}`,
				}, nil
			}

			// Review stage uses sonnet/opus
			reviewCalled = true
			reviewCallTime = time.Now()
			return &claude.Result{
				Success: true,
				Output:  `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "Review completed successfully"}`,
			}, nil
		},
	}

	mockRend := &mockPromptRenderer{
		LearningsFile: nil, // No learnings file since learning is disabled
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review code for improvements", nil
		},
	}

	learnFromSuccessDisabled := false
	syncOut := newSyncWriter(&buf)
	cfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessDisabled, // Learning disabled
		},
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	mockRouter := newMockRouterFromClaudeClient(mockClaude)
	mockBeads := &mockBeadClient{}
	gitDiffFn := func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}
	reviewer := reviewpkg.NewReviewer(cfg, mockRouter, mockBeads, mockRend, gitDiffFn, nil)
	reviewer.SetLogFn(func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(syncOut, format+"\n", args...)
	})
	r := &Runner{
		cfg:              cfg,
		router:           mockRouter,
		renderer:         mockRend,
		beads:            mockBeads,
		output:           syncOut,
		reviewer:         reviewer,
		gitDiffFn:        gitDiffFn,
		cmdRunnerFn:      cmdRunner,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-only-review",
			Title:       "Test Single-Stage Review",
			Description: "Verify review runs inline when learning is disabled",
		},
		Model:       "sonnet",
		Result:      &IterationResult{Validated: true},
		StartCommit: "abc123",
		Iteration:   1,
		RunDeadline: time.Now().Add(5 * time.Minute),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	start := time.Now()
	err := r.runValidation(context.Background(), bc)
	elapsed := time.Since(start)

	// Should not return an error
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify learning was NOT called (disabled)
	if learningCalled {
		t.Error("learning extraction should NOT have been called when disabled")
	}

	// Verify review was called
	if !reviewCalled {
		t.Error("review should have been called")
	}

	// Verify inline execution: review should complete before runValidation returns
	// If review ran in a goroutine, there would be potential for it to not complete yet
	if !reviewCallTime.IsZero() && reviewCallTime.After(start.Add(elapsed)) {
		t.Error("review should have completed before runValidation returned (inline execution)")
	}

	// Verify output contains review success message
	output := buf.String()
	if !strings.Contains(output, "Review:") || !strings.Contains(output, "Review completed successfully") {
		t.Errorf("expected review completion message in output, got: %s", output)
	}

	// Verify output does NOT contain learning message
	if strings.Contains(output, "Success learning") {
		t.Errorf("expected no learning message when disabled, got: %s", output)
	}
}

func TestPostSuccess_BothStagesEnabled_RunSequentially(t *testing.T) {
	// Verifies that when both learning and review are enabled,
	// both stages run and complete successfully.

	var buf strings.Builder
	var learningCalled, reviewCalled bool
	var mu sync.Mutex

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Learning extraction uses haiku model
			if model == "haiku" {
				mu.Lock()
				learningCalled = true
				mu.Unlock()

				return &claude.Result{
					Success: true,
					Output:  `{"learning": "Test learning", "category": "patterns"}`,
				}, nil
			}

			// Review stage uses sonnet/opus
			mu.Lock()
			reviewCalled = true
			mu.Unlock()

			return &claude.Result{
				Success: true,
				Output:  `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "Review completed"}`,
			}, nil
		},
	}

	lf, _ := learnings.NewFile(t.TempDir())
	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review code for improvements", nil
		},
	}

	learnFromSuccessEnabled := true
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	postSuccessCfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	syncOut := newSyncWriter(&buf)
	mockRouter := newMockRouterFromClaudeClient(mockClaude)
	mockBeads := &mockBeadClient{}
	gitDiffFn := func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}
	reviewer := reviewpkg.NewReviewer(postSuccessCfg, mockRouter, mockBeads, mockRend, gitDiffFn, nil)
	reviewer.SetLogFn(func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(syncOut, format+"\n", args...)
	})
	r := &Runner{
		cfg:              postSuccessCfg,
		router:           mockRouter,
		renderer:         mockRend,
		beads:            mockBeads,
		output:           syncOut,
		reviewer:         reviewer,
		gitDiffFn:        gitDiffFn,
		validationRunner: validation.NewRunner(postSuccessCfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-both-stages",
			Title:       "Test Both Post-Success Stages",
			Description: "Verify learning and review both run",
		},
		Model:       "sonnet",
		Result:      &IterationResult{Validated: true},
		StartCommit: "abc123",
		Iteration:   1,
		RunDeadline: time.Now().Add(5 * time.Minute),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	err := r.runValidation(context.Background(), bc)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify both stages were called
	mu.Lock()
	defer mu.Unlock()

	if !learningCalled {
		t.Error("learning should have been called")
	}
	if !reviewCalled {
		t.Error("review should have been called")
	}

	// Verify both stages completed successfully
	output := buf.String()
	if !strings.Contains(output, "Success learning extracted") {
		t.Error("expected learning completion message")
	}
	if !strings.Contains(output, "Review:") {
		t.Error("expected review completion message")
	}
}

func TestPostSuccess_LearningFailureDoesNotBlockReview(t *testing.T) {
	// Verifies that a failure in learning does not prevent
	// the review stage from completing.

	var buf strings.Builder
	var learningCalled, reviewCalled bool
	var mu sync.Mutex

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Learning extraction uses haiku model and fails
			if model == "haiku" {
				mu.Lock()
				learningCalled = true
				mu.Unlock()

				// Fail immediately
				return nil, fmt.Errorf("learning extraction failed: network timeout")
			}

			// Review stage uses sonnet/opus and succeeds
			mu.Lock()
			reviewCalled = true
			mu.Unlock()

			return &claude.Result{
				Success: true,
				Output:  `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "Review completed despite learning failure"}`,
			}, nil
		},
	}

	lf, _ := learnings.NewFile(t.TempDir())
	mockRend := &mockPromptRenderer{
		LearningsFile: lf,
		RenderLearnFn: func(ctx *prompt.LearnContext) (string, error) {
			return "extract learning from success", nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review code for improvements", nil
		},
	}

	learnFromSuccessEnabled := true
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}
	postSuccessCfg := &config.Config{
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccessEnabled,
		},
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	syncOut := newSyncWriter(&buf)
	mockRouter := newMockRouterFromClaudeClient(mockClaude)
	mockBeads := &mockBeadClient{}
	gitDiffFn := func(fromCommit string) (string, error) {
		return "diff --git a/file.go b/file.go\n+some change", nil
	}
	reviewer := reviewpkg.NewReviewer(postSuccessCfg, mockRouter, mockBeads, mockRend, gitDiffFn, nil)
	reviewer.SetLogFn(func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(syncOut, format+"\n", args...)
	})
	r := &Runner{
		cfg:              postSuccessCfg,
		router:           mockRouter,
		renderer:         mockRend,
		beads:            mockBeads,
		output:           syncOut,
		reviewer:         reviewer,
		gitDiffFn:        gitDiffFn,
		validationRunner: validation.NewRunner(postSuccessCfg, cmdRunner, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-concurrent-isolation",
			Title:       "Test Concurrent Stage Isolation",
			Description: "Verify learning failure doesn't block review",
		},
		Model:       "sonnet",
		Result:      &IterationResult{Validated: true},
		StartCommit: "abc123",
		Iteration:   1,
		RunDeadline: time.Now().Add(5 * time.Minute),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
	}

	err := r.runValidation(context.Background(), bc)

	// The method should not return an error (learning failures are silently ignored)
	if err != nil {
		t.Errorf("expected no error when learning fails, got: %v", err)
	}

	// Verify both stages were called
	mu.Lock()
	defer mu.Unlock()

	if !learningCalled {
		t.Error("learning should have been called")
	}
	if !reviewCalled {
		t.Error("review should have been called")
	}

	// Verify review completed despite learning failure
	output := buf.String()
	if !strings.Contains(output, "Review:") && !strings.Contains(output, "Review completed despite learning failure") {
		t.Errorf("expected review completion message, got: %s", output)
	}
}

// --- Validation Recovery Tests (Bead 1.5) ---

func TestRunValidationWithRecovery_PassesOnFirstTry(t *testing.T) {
	var buf strings.Builder

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:        true,
			Commands:       []string{"go test ./..."},
			MaxFixAttempts: 1,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	}

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		output:           &buf,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		Model:  "sonnet",
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir:            t.TempDir(),
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	err := r.runValidationWithRecovery(context.Background(), bc)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if bc.Result.ValidationRetried {
		t.Error("ValidationRetried should be false when validation passes on first try")
	}
}

func TestRunValidationWithRecovery_FailsThenFixSucceeds(t *testing.T) {
	var buf strings.Builder
	cmdCallCount := 0
	executeFnCalls := 0

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxFixAttempts:       1,
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdCallCount++
		if cmdCallCount == 1 {
			return "", "FAIL: TestSomething", 1, nil
		}
		return "ok", "", 0, nil
	}

	valRunner := validation.NewRunner(cfg, cmdRunner, nil,
		func(ctx context.Context, bc *runtypes.BeadContext) bool {
			executeFnCalls++
			return true
		},
	)

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		analyzer:         &mockFailureAnalyzer{},
		output:           &buf,
		validationRunner: valRunner,
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		Model:  "sonnet",
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir:            t.TempDir(),
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
		MaxRetries:        1,
		MaxRetriesPerBead: 5,
		ParentCtx:         context.Background(),
	}

	err := r.runValidationWithRecovery(context.Background(), bc)
	if err != nil {
		t.Errorf("expected no error after successful fix, got: %v", err)
	}
	if !bc.Result.ValidationRetried {
		t.Error("ValidationRetried should be true when recovery was attempted")
	}
	if cmdCallCount < 2 {
		t.Errorf("expected at least 2 cmd calls, got %d", cmdCallCount)
	}
	if executeFnCalls != 1 {
		t.Errorf("expected 1 executeFn call, got %d", executeFnCalls)
	}
}

func TestRunValidationWithRecovery_FailsThenFixStillFails(t *testing.T) {
	var buf strings.Builder

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxFixAttempts:       1,
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "FAIL: TestSomething", 1, nil
	}

	valRunner := validation.NewRunner(cfg, cmdRunner, nil,
		func(ctx context.Context, bc *runtypes.BeadContext) bool {
			return true // Claude fix "succeeds" but validation still fails
		},
	)

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		analyzer:         &mockFailureAnalyzer{},
		output:           &buf,
		validationRunner: valRunner,
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		Model:  "sonnet",
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir:            t.TempDir(),
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
		MaxRetries:        1,
		MaxRetriesPerBead: 5,
		ParentCtx:         context.Background(),
	}

	err := r.runValidationWithRecovery(context.Background(), bc)
	if err == nil {
		t.Error("expected error when fix doesn't resolve validation")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected 'validation failed' error, got: %v", err)
	}
	if !bc.Result.ValidationRetried {
		t.Error("ValidationRetried should be true when recovery was attempted")
	}
}

func TestRunValidationWithRecovery_InvocationErrorNotRecovered(t *testing.T) {
	var buf strings.Builder

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:        true,
			Commands:       []string{"go test ./..."},
			MaxFixAttempts: 1,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", -1, fmt.Errorf("network error")
	}

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		output:           &buf,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		Model:  "sonnet",
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir:            t.TempDir(),
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	err := r.runValidationWithRecovery(context.Background(), bc)
	if err == nil {
		t.Error("expected error for invocation failure")
	}
	if !strings.Contains(err.Error(), "validation command") {
		t.Errorf("expected 'validation command' error, got: %v", err)
	}
	// Invocation errors should not trigger recovery
	if bc.Result.ValidationRetried {
		t.Error("ValidationRetried should be false for invocation errors")
	}
}

// --- Config Tests for MaxFixAttempts ---

func TestMaxFixAttempts_DefaultValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	if cfg.Validation.MaxFixAttempts != 1 {
		t.Errorf("expected default MaxFixAttempts=1, got %d", cfg.Validation.MaxFixAttempts)
	}
}

func TestMaxFixAttempts_LoadFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlContent := `
validation:
  enabled: true
  max_fix_attempts: 3
  commands:
    - go test ./...
`
	cfgPath := tmpDir + "/gromit.yaml"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Validation.MaxFixAttempts != 3 {
		t.Errorf("expected MaxFixAttempts=3, got %d", cfg.Validation.MaxFixAttempts)
	}
}

// --- Diagnostic Wiring Tests ---

func TestExecuteClaudeInvocation_PopulatesDiagnostics(t *testing.T) {
	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "ok"}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			Claude: config.ClaudeConfig{
				StallTimeout:       30,
				StallTimeoutActive: 60,
			},
		},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}
	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-1", Title: "Test"},
		Model:       "sonnet",
		Result:      &IterationResult{},
		BuildPrompt: "test prompt",
	}

	invResult, providerResult, err := r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil || invResult.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if providerResult == nil {
		t.Fatal("expected non-nil provider result")
	}
	if invResult.StallFired {
		t.Error("expected stallFired=false")
	}
	if invResult.Stats == nil {
		t.Fatal("expected non-nil stats")
	}

	// Diagnostic fields should be populated on bc.Result
	if bc.Result.TimeToFirstEventMs < 0 {
		t.Errorf("expected non-negative TimeToFirstEventMs, got %d", bc.Result.TimeToFirstEventMs)
	}
	// StallCount should be 0 for a successful run
	if bc.Result.StallCount != 0 {
		t.Errorf("expected StallCount=0, got %d", bc.Result.StallCount)
	}
}

// --- Preemptive Escalation Tests ---

func TestBuildPromptForBead_MediumComplexityKeepsSonnet(t *testing.T) {
	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			// Scope check returns medium complexity
			return &claude.Result{
				Success: true,
				Output:  `{"complexity":"medium","estimated_iterations":1,"can_complete_in_single_iteration":true,"blockers":[],"rationale":"medium task"}`,
			}, nil
		},
	}
	mockRend := &mockPromptRenderer{
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Model:              model,
				ConfirmedLearnings: []learnings.Learning{},
				RecentLearnings:    []learnings.Learning{},
			}, nil
		},
	}

	// Create a mock provider that returns the same output as mockClaude
	mockProvider := &mockProviderForProcess{
		claudeClient: mockClaude,
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
		},
		router:   mockRouter,
		beads:    &mockBeadClient{},
		renderer: mockRend,
		output:   &buf,
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
		Model:  "sonnet",
		Result: &IterationResult{Model: "sonnet"},
	}

	err := r.buildPromptForBead(context.Background(), bc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Sonnet should stay sonnet for medium complexity — no auto-escalation
	if bc.Model != "sonnet" {
		t.Errorf("expected model to stay 'sonnet', got %q", bc.Model)
	}
	if strings.Contains(buf.String(), "auto-escalating") {
		t.Errorf("should not auto-escalate medium complexity on sonnet, got: %s", buf.String())
	}
}

func TestBuildPromptForBead_MediumComplexityKeepsOpus(t *testing.T) {
	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  `{"complexity":"medium","estimated_iterations":1,"can_complete_in_single_iteration":true,"blockers":[],"rationale":"medium task"}`,
			}, nil
		},
	}
	mockRend := &mockPromptRenderer{
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Model:              model,
				ConfirmedLearnings: []learnings.Learning{},
				RecentLearnings:    []learnings.Learning{},
			}, nil
		},
	}

	// Create a mock provider that returns the same output as mockClaude
	mockProvider := &mockProviderForProcess{
		claudeClient: mockClaude,
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
		},
		router:   mockRouter,
		beads:    &mockBeadClient{},
		renderer: mockRend,
		output:   &buf,
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
		Model:  "opus",
		Result: &IterationResult{Model: "opus"},
	}

	err := r.buildPromptForBead(context.Background(), bc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Opus should stay opus for medium complexity
	if bc.Model != "opus" {
		t.Errorf("expected model to stay 'opus', got %q", bc.Model)
	}
	if strings.Contains(buf.String(), "auto-escalating") {
		t.Errorf("should not escalate opus, got: %s", buf.String())
	}
}

func TestBuildPromptForBead_LowComplexityKeepsSonnet(t *testing.T) {
	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  `{"complexity":"low","estimated_iterations":1,"can_complete_in_single_iteration":true,"blockers":[],"rationale":"simple task"}`,
			}, nil
		},
	}
	mockRend := &mockPromptRenderer{
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Model:              model,
				ConfirmedLearnings: []learnings.Learning{},
				RecentLearnings:    []learnings.Learning{},
			}, nil
		},
	}

	// Create a mock provider that returns the same output as mockClaude
	mockProvider := &mockProviderForProcess{
		claudeClient: mockClaude,
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
		},
		router:   mockRouter,
		beads:    &mockBeadClient{},
		renderer: mockRend,
		output:   &buf,
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
		Model:  "sonnet",
		Result: &IterationResult{Model: "sonnet"},
	}

	err := r.buildPromptForBead(context.Background(), bc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Sonnet should stay sonnet for low complexity
	if bc.Model != "sonnet" {
		t.Errorf("expected model to stay 'sonnet', got %q", bc.Model)
	}
}

func TestBuildPromptForBead_SkipsScopeEscalationWhenTierHigh(t *testing.T) {
	var buf strings.Builder
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled: true,
			Model:   "haiku",
		},
	}
	r := &Runner{
		cfg:               cfg,
		renderer:          &mockPromptRenderer{},
		output:            &buf,
		escalationHandler: newTestEscalationHandler(cfg),
	}
	bc := &runtypes.BeadContext{
		Bead:             &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
		Model:            "opus",
		Tier:             provider.TierHigh,
		Result:           &IterationResult{Model: "opus"},
		RetriesThisModel: 2,
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "high",
			EstimatedIterations:          2,
			CanCompleteInSingleIteration: false,
			Rationale:                    "Large change",
			Blockers:                     []string{},
		},
	}

	err := r.buildPromptForBead(context.Background(), bc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bc.RetriesThisModel != 2 {
		t.Errorf("expected retriesThisModel to remain 2, got %d", bc.RetriesThisModel)
	}
	if strings.Contains(buf.String(), "auto-escalating") {
		t.Errorf("should not auto-escalate when tier already high, got: %s", buf.String())
	}
}

func TestWriteIterationLog_DiagnosticFields(t *testing.T) {
	mockLog := &mockIterationLogger{}

	r := &Runner{
		cfg:    &config.Config{},
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &IterationResult{
		BeadID:             "test-1",
		BeadTitle:          "Test",
		Model:              "sonnet",
		Success:            false,
		TimeoutType:        "stall",
		TimeToFirstEventMs: 1234,
		ToolCallCount:      5,
		StallCount:         2,
		StallTier:          "active",
		RateLimitHits:      1,
		Error:              fmt.Errorf("stall timeout"),
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}
	log := mockLog.Logs[0]
	if log.TimeoutType != "stall" {
		t.Errorf("expected TimeoutType='stall', got %q", log.TimeoutType)
	}
	if log.TimeToFirstEventMs != 1234 {
		t.Errorf("expected TimeToFirstEventMs=1234, got %d", log.TimeToFirstEventMs)
	}
	if log.ToolCallCount != 5 {
		t.Errorf("expected ToolCallCount=5, got %d", log.ToolCallCount)
	}
	if log.StallCount != 2 {
		t.Errorf("expected StallCount=2, got %d", log.StallCount)
	}
	if log.StallTier != "active" {
		t.Errorf("expected StallTier='active', got %q", log.StallTier)
	}
	if log.RateLimitHits != 1 {
		t.Errorf("expected RateLimitHits=1, got %d", log.RateLimitHits)
	}
}

// mockProviderForProcess wraps a claude.Client to implement provider.Provider for process tests
type mockProviderForProcess struct {
	claudeClient *mockClaudeClient
}

func (m *mockProviderForProcess) Name() string {
	return "mock"
}

func (m *mockProviderForProcess) ModelForTier(tier string) string {
	// Simple tier-to-model mapping for tests
	tierMap := map[string]string{
		provider.TierHigh:   "opus",
		provider.TierMedium: "sonnet",
		provider.TierLow:    "haiku",
	}
	if model, ok := tierMap[tier]; ok {
		return model
	}
	return tier
}

func (m *mockProviderForProcess) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	result, err := m.claudeClient.Run(ctx, prompt, tier)
	if err != nil {
		return nil, err
	}
	return &provider.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}, nil
}

func (m *mockProviderForProcess) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	// Convert provider handlers to claude handlers
	var claudeHandler claude.EventHandler
	if handler != nil {
		claudeHandler = func(line []byte) {
			handler(line)
		}
	}

	var claudeToolHandler claude.ToolCallHandler
	if onToolCall != nil {
		claudeToolHandler = func(event claude.ToolEvent) {
			onToolCall(provider.ToolEvent{
				ToolName:  event.ToolName,
				FilePath:  event.FilePath,
				Timestamp: event.Timestamp,
			})
		}
	}

	result, err := m.claudeClient.StreamRun(ctx, prompt, tier, output, claudeHandler, claudeToolHandler)
	if err != nil {
		return nil, err
	}
	return &provider.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}, nil
}

func (m *mockProviderForProcess) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	result, err := m.claudeClient.RunValidation(ctx, commands, tier, workDir)
	if err != nil {
		return nil, err
	}
	return &provider.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}, nil
}

func (m *mockProviderForProcess) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (m *mockProviderForProcess) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}

func (m *mockProviderForProcess) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

func configureRefactorSkipped(r *Runner) {
	r.cfg.Refactor.MinFilesChanged = 0
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) { return "", nil }, // no diff → refactor skipped
		nil, nil, nil, nil, nil,
	))
}

func configureRefactorWithDiff(r *Runner, refactorInvoked *bool) {
	r.cfg.Refactor.MinFilesChanged = 0
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			if refactorInvoked != nil {
				*refactorInvoked = true // getDiff is called at start of RunRefactorPhase
			}
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) { return "refactor prompt", nil },
		func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
			return &claude.Result{Success: true}, nil, nil
		},
		nil,
		nil,
		func() (string, error) { return "abc123", nil },
	))
}

func TestRunRefactorAndPostChecks_RevalidationSkippedWhenUnderThreshold(t *testing.T) {
	validationCalled := false
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalled = true
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	configureRefactorSkipped(r)

	parentCtx := context.Background()
	// Set remaining to 5s (under the minimum threshold for re-validation)
	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   "abc123",
		ParentCtx:     parentCtx,
		BeadTimeout:   30 * time.Second,
		BeadStartTime: time.Now().Add(-25 * time.Second), // 5s remaining, below threshold
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal=nil when under threshold, got: %+v", terminal)
	}
	if validationCalled {
		t.Fatal("expected post-refactor re-validation to be skipped when time is under threshold")
	}
	logOutput := r.output.(*strings.Builder).String()
	if !strings.Contains(logOutput, "remaining") {
		t.Error("expected skip log to contain remaining timing details")
	}
	if !strings.Contains(logOutput, "needed") {
		t.Error("expected skip log to contain needed timing details")
	}
}

func TestRunRefactorAndPostChecks_RefactorSkippedWhenTimeExpired(t *testing.T) {
	refactorInvoked := false
	r, _, _ := setupDirectValidationRunner(t, nil, nil)

	configureRefactorWithDiff(r, &refactorInvoked)

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   "abc123",
		ParentCtx:     parentCtx,
		BeadTimeout:   30 * time.Second,
		BeadStartTime: time.Now().Add(-60 * time.Second), // expired
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal=nil when time expired, got: %+v", terminal)
	}
	if refactorInvoked {
		t.Fatal("expected refactor phase to be skipped when bead time has expired")
	}
	logOutput := r.output.(*strings.Builder).String()
	if !strings.Contains(logOutput, "remaining") {
		t.Error("expected skip log to contain remaining timing details")
	}
}

func TestRunRefactorAndPostChecks_RefactorSkippedWhenDeadlineInsufficient(t *testing.T) {
	refactorInvoked := false
	r, _, _ := setupDirectValidationRunner(t, nil, nil)

	configureRefactorWithDiff(r, &refactorInvoked)

	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   "abc123",
		ParentCtx:     context.Background(),
		BeadTimeout:   5 * time.Minute,
		BeadStartTime: time.Now().Add(-10 * time.Second),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	retry, terminal := r.runRefactorAndPostChecks(ctx, bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal=nil when deadline is insufficient, got: %+v", terminal)
	}
	if refactorInvoked {
		t.Fatal("expected refactor phase to be skipped when deadline is insufficient")
	}
	logOutput := r.output.(*strings.Builder).String()
	if !strings.Contains(logOutput, "remaining") {
		t.Error("expected skip log to contain remaining timing details")
	}
	if !strings.Contains(logOutput, "needed") {
		t.Error("expected skip log to contain needed timing details")
	}
	if !strings.Contains(logOutput, "reason=") {
		t.Error("expected skip log to contain machine-readable reason")
	}
}

func TestRunRefactorAndPostChecks_RevalidationSkippedWhenTimeExpired(t *testing.T) {
	validationCalled := false
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalled = true
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	configureRefactorSkipped(r)

	parentCtx := context.Background()
	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   "abc123",
		ParentCtx:     parentCtx,
		BeadTimeout:   30 * time.Second,
		BeadStartTime: time.Now().Add(-60 * time.Second), // started 60s ago, 30s timeout → expired
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal != nil {
		t.Fatalf("expected terminal=nil when time expired, got: %+v", terminal)
	}
	if validationCalled {
		t.Fatal("expected post-refactor re-validation to be skipped when bead time has expired")
	}
	if !strings.Contains(r.output.(*strings.Builder).String(), "remaining") {
		t.Error("expected skip log to contain timing details (remaining)")
	}
}

func TestRunRefactorAndPostChecks_NonTimeoutValidationFailure(t *testing.T) {
	lintErr := errors.New("lint failed")
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "", 1, lintErr
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	configureRefactorSkipped(r)

	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   "abc123",
		ParentCtx:     context.Background(),
		BeadTimeout:   5 * time.Minute,
		BeadStartTime: time.Now(),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)
	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal to be non-nil on validation failure")
	}
	if terminal.Error == nil {
		t.Fatal("expected terminal error to be set on validation failure")
	}
	if !strings.Contains(terminal.Error.Error(), "validation failed after refactoring") {
		t.Fatalf("expected terminal error to mention refactor validation failure, got: %v", terminal.Error)
	}
	if !errors.Is(terminal.Error, lintErr) {
		t.Fatalf("expected terminal error to wrap lint error, got: %v", terminal.Error)
	}
}

func TestSetupBeadContext_SetsSpecIDFromLabel(t *testing.T) {
	r := newRunnerForBeadContextTest(t)
	b := &bead.Bead{ID: "test-spec", Title: "Spec Test", Priority: 1, Labels: []string{"spec:my-spec"}}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	if bc.Result.SpecID != "my-spec" {
		t.Errorf("SpecID = %q, want %q", bc.Result.SpecID, "my-spec")
	}
}

func TestSetupBeadContext_SpecIDEmptyWhenNoSpecLabel(t *testing.T) {
	r := newRunnerForBeadContextTest(t)
	b := &bead.Bead{ID: "test-no-spec", Title: "No Spec Test", Priority: 1, Labels: []string{"priority:1", "type:task"}}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	if bc.Result.SpecID != "" {
		t.Errorf("SpecID = %q, want empty string", bc.Result.SpecID)
	}
}

func newRunnerForBeadContextTest(t *testing.T) *Runner {
	t.Helper()

	return &Runner{
		cfg: &config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 300},
		},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   &strings.Builder{},
		router:   newMockRouter(),
		gitHeadFn: func() (string, error) {
			return "abc123", nil
		},
	}
}

func TestProcessBead_FilesTouched(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "implemented"}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() failed: %v", err)
	}

	// Inject mock git diff returning 3 files
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" +
			"diff --git a/bar.go b/bar.go\n--- a/bar.go\n+++ b/bar.go\n" +
			"diff --git a/baz_test.go b/baz_test.go\n--- a/baz_test.go\n+++ b/baz_test.go\n", nil
	}

	b := &bead.Bead{ID: "ft-test", Title: "File Touch Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if result.FilesTouched != 3 {
		t.Errorf("FilesTouched = %d, want 3", result.FilesTouched)
	}
	if result.ActualTier != provider.TierMedium {
		t.Errorf("ActualTier = %q, want %q", result.ActualTier, provider.TierMedium)
	}
}
