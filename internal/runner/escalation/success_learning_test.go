package escalation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Mock types for success learning extraction ---

type mockSuccessProvider struct {
	runFn func(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error)
}

func (m *mockSuccessProvider) Run(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &mockSuccessResult{success: true, output: `{"learning": "test learning", "category": "patterns"}`}, nil
}

type mockSuccessRouter struct {
	selectFn func(phase, tier string) (SuccessLearningProvider, string)
}

func (m *mockSuccessRouter) Select(phase, tier string) (SuccessLearningProvider, string) {
	if m.selectFn != nil {
		return m.selectFn(phase, tier)
	}
	return &mockSuccessProvider{}, "haiku"
}

type mockSuccessResult struct {
	success bool
	output  string
}

func (m *mockSuccessResult) IsSuccess() bool   { return m.success }
func (m *mockSuccessResult) GetOutput() string { return m.output }

// --- ExtractSuccessLearning behavioral tests ---

// TestExtractSuccessLearning_PersistsLearningForMediumTier verifies that
// ExtractSuccessLearning invokes the router to get a provider, calls
// the provider with the rendered prompt, parses the response, and
// persists the extracted learning to the learnings file.
//
// Expected failure: ExtractSuccessLearning is currently a placeholder that
// accepts untyped interface{} params for router and renderer and does nothing
// beyond skipping low-tier beads. The full implementation needs to accept
// typed narrow interfaces (SuccessLearningRouter, SuccessLearningRenderer),
// render a prompt, call the provider, parse the JSON response, and
// call lf.Add() to persist the learning.
func TestExtractSuccessLearning_PersistsLearningForMediumTier(t *testing.T) {
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "success-med-001",
			Title:       "Add user validation",
			Description: "Adds input validation to the user registration form",
		},
		Tier:   provider.TierMedium,
		Model:  "sonnet",
		Result: &runtypes.IterationResult{},
	}

	learningText := "User input validation should always happen at the API boundary"
	router := &mockSuccessRouter{
		selectFn: func(phase, tier string) (SuccessLearningProvider, string) {
			return &mockSuccessProvider{
				runFn: func(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error) {
					return &mockSuccessResult{
						success: true,
						output:  `{"learning": "` + learningText + `", "category": "conventions"}`,
					}, nil
				},
			}, "haiku"
		},
	}

	// Call ExtractSuccessLearning with typed interfaces.
	// After implementation, this function should accept SuccessLearningRouter
	// (not interface{}) and actually use it to extract and persist the learning.
	ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil)

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if !strings.Contains(string(content), learningText) {
		t.Errorf("learnings file should contain extracted learning %q, got:\n%s", learningText, string(content))
	}
}

// TestExtractSuccessLearning_SkipsWhenRouterReturnsNilProvider verifies that
// ExtractSuccessLearning is a no-op when the router returns a nil provider
// (no providers available for learning extraction).
//
// Expected failure: ExtractSuccessLearning is currently a placeholder that
// ignores the router parameter entirely. After implementation, it should
// call router.Select() and gracefully skip when the provider is nil.
func TestExtractSuccessLearning_SkipsWhenRouterReturnsNilProvider(t *testing.T) {
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "success-nil-001", Title: "Test bead"},
		Tier:   provider.TierMedium,
		Model:  "sonnet",
		Result: &runtypes.IterationResult{},
	}

	routerCalled := false
	router := &mockSuccessRouter{
		selectFn: func(phase, tier string) (SuccessLearningProvider, string) {
			routerCalled = true
			return nil, "" // No provider available
		},
	}

	ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil)

	if !routerCalled {
		t.Error("ExtractSuccessLearning should call router.Select() to get a provider — currently ignores the router parameter")
	}
}

// TestExtractSuccessLearning_SkipsWhenProviderFails verifies that
// ExtractSuccessLearning does not persist anything when the provider
// returns an error (learning extraction is optional).
//
// Expected failure: ExtractSuccessLearning is currently a placeholder that
// never calls the provider. After implementation, it should call the
// provider and gracefully skip when the call fails.
func TestExtractSuccessLearning_SkipsWhenProviderFails(t *testing.T) {
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "success-fail-001", Title: "Test bead"},
		Tier:   provider.TierMedium,
		Model:  "sonnet",
		Result: &runtypes.IterationResult{},
	}

	providerCalled := false
	router := &mockSuccessRouter{
		selectFn: func(phase, tier string) (SuccessLearningProvider, string) {
			return &mockSuccessProvider{
				runFn: func(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error) {
					providerCalled = true
					return nil, context.DeadlineExceeded
				},
			}, "haiku"
		},
	}

	ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil)

	if !providerCalled {
		t.Error("ExtractSuccessLearning should call the provider to extract a learning — currently never calls the provider")
	}

	// Verify nothing was persisted
	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if strings.Contains(string(content), "success-fail-001") {
		t.Error("should not persist learning when provider fails")
	}
}

// TestExtractSuccessLearning_SkipsWhenProviderReturnsNoLearning verifies that
// ExtractSuccessLearning does not persist anything when the provider
// returns a successful result but with no learning (null learning field).
//
// Expected failure: ExtractSuccessLearning is currently a placeholder.
// After implementation, it should parse the provider response and skip
// when the learning field is null.
func TestExtractSuccessLearning_SkipsWhenProviderReturnsNoLearning(t *testing.T) {
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "success-nolearn-001", Title: "Trivial bead"},
		Tier:   provider.TierMedium,
		Model:  "sonnet",
		Result: &runtypes.IterationResult{},
	}

	providerCalled := false
	router := &mockSuccessRouter{
		selectFn: func(phase, tier string) (SuccessLearningProvider, string) {
			return &mockSuccessProvider{
				runFn: func(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error) {
					providerCalled = true
					return &mockSuccessResult{
						success: true,
						output:  `{"learning": null, "category": ""}`,
					}, nil
				},
			}, "haiku"
		},
	}

	ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil)

	if !providerCalled {
		t.Error("ExtractSuccessLearning should call the provider — currently never calls it")
	}

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if strings.Contains(string(content), "success-nolearn-001") {
		t.Error("should not persist learning when provider returns null learning")
	}
}

// TestExtractSuccessLearning_HighTierUsesRouter verifies that high-tier beads
// (opus) also get success learning extraction through the router.
//
// Expected failure: ExtractSuccessLearning is currently a placeholder.
// After implementation, high-tier beads should follow the same
// router → provider → parse → persist path.
func TestExtractSuccessLearning_HighTierUsesRouter(t *testing.T) {
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:    "success-high-001",
			Title: "Complex feature implementation",
		},
		Tier:   provider.TierHigh,
		Model:  "opus",
		Result: &runtypes.IterationResult{},
	}

	learningText := "Complex features benefit from iterative decomposition"
	routerCalled := false
	router := &mockSuccessRouter{
		selectFn: func(phase, tier string) (SuccessLearningProvider, string) {
			routerCalled = true
			return &mockSuccessProvider{
				runFn: func(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error) {
					return &mockSuccessResult{
						success: true,
						output:  `{"learning": "` + learningText + `", "category": "patterns"}`,
					}, nil
				},
			}, "haiku"
		},
	}

	ExtractSuccessLearning(context.Background(), bc, cfg, lf, router, nil)

	if !routerCalled {
		t.Error("ExtractSuccessLearning should call router.Select() for high-tier beads — currently ignores the router")
	}

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if !strings.Contains(string(content), learningText) {
		t.Errorf("learnings file should contain %q for high-tier bead, got:\n%s", learningText, string(content))
	}
}
