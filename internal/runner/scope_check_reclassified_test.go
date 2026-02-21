package runner

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestScopeCheckReclassified_CachedEstimateSkipsDuplicateInvocation verifies
// that buildPromptForBead uses a pre-populated bc.ScopeEstimate from the scope
// gate without issuing a second checkScope (RenderScope) call. When an estimate
// is already cached on the BeadContext, no additional provider invocation should
// occur. This is the unit-level reclassification of the acceptance-level scope
// deduplication check.
func TestScopeCheckReclassified_CachedEstimateSkipsDuplicateInvocation(t *testing.T) {
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled: true,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:     300,
			AnalysisTimeout: 60,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	enableScope := true
	cfg.ScopeCheck.Enabled = enableScope

	cachedEstimate := &prompt.ScopeEstimate{
		Complexity:                   "medium",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	var renderScopeMu sync.Mutex
	renderScopeCallCount := 0

	mockRenderer := &mockPromptRenderer{
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			renderScopeMu.Lock()
			renderScopeCallCount++
			renderScopeMu.Unlock()
			data, err := json.Marshal(cachedEstimate)
			if err != nil {
				t.Fatalf("json.Marshal scope estimate: %v", err)
			}
			return string(data), nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
		},
	}

	mockBeads := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: mockRenderer,
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	testBead := &bead.Bead{
		ID:              "reclassified-scope-test",
		Title:           "Scope cache unit test",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Construct a BeadContext with the estimate already populated (simulating
	// the result of a prior scope gate check).
	bc := &runtypes.BeadContext{
		Bead:          testBead,
		Parent:        nil,
		Result:        &IterationResult{Model: "sonnet"},
		Model:         "sonnet",
		ScopeEstimate: cachedEstimate, // pre-cached — must not trigger a second call
	}

	// buildPromptForBead must consume the cached estimate without calling checkScope.
	buildErr := r.buildPromptForBead(context.Background(), bc, 1)
	if buildErr != nil {
		t.Fatalf("buildPromptForBead() unexpected error: %v", buildErr)
	}

	renderScopeMu.Lock()
	callsAfterBuild := renderScopeCallCount
	renderScopeMu.Unlock()

	// No RenderScope call should have been issued because the estimate was cached.
	if callsAfterBuild != 0 {
		t.Errorf("buildPromptForBead called RenderScope %d time(s) despite "+
			"bc.ScopeEstimate being pre-populated; expected 0 additional calls",
			callsAfterBuild)
	}
}
