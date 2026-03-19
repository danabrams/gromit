package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/provider"
)

// Compile-time interface check.
var _ ReviewAgent = (*ProviderReviewAgent)(nil)

// ProviderReviewAgent satisfies the ReviewAgent interface by delegating to an
// llmadapter.Invoker. It extracts JSON from LLM output and unmarshals it into
// []Finding values.
type ProviderReviewAgent struct {
	invoker   llmadapter.Invoker
	workDirFn func() string
}

// NewProviderReviewAgent creates a ProviderReviewAgent that delegates to the given invoker.
func NewProviderReviewAgent(invoker llmadapter.Invoker) *ProviderReviewAgent {
	return &ProviderReviewAgent{invoker: invoker}
}

// NewProviderReviewAgentWithDir creates a ProviderReviewAgent that uses InvokeInDir
// with the directory returned by workDirFn. The workDirFn is a lazy resolver
// because the worktree path isn't known at agent construction time.
func NewProviderReviewAgentWithDir(invoker llmadapter.Invoker, workDirFn func() string) *ProviderReviewAgent {
	return &ProviderReviewAgent{invoker: invoker, workDirFn: workDirFn}
}

// ReviewFacet delegates to the invoker, extracts JSON from the output, and
// unmarshals it into []Finding. The facetName parameter is for logging/labeling
// only and is not passed to the invoker.
func (a *ProviderReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	var result *provider.Result
	var err error
	if a.workDirFn != nil {
		if dir := a.workDirFn(); dir != "" {
			result, err = a.invoker.InvokeInDir(ctx, prompt, dir)
		} else {
			result, err = a.invoker.Invoke(ctx, prompt)
		}
	} else {
		result, err = a.invoker.Invoke(ctx, prompt)
	}
	if err != nil {
		return []Finding{}, err
	}
	if result == nil {
		return []Finding{}, fmt.Errorf("review: provider returned nil result")
	}

	extracted := llmadapter.ExtractJSON(result.Output)

	// If extraction returned no JSON (pure prose), return as ParseError for retry.
	trimmed := strings.TrimSpace(extracted)
	if trimmed == "" || (trimmed[0] != '[' && trimmed[0] != '{') {
		return []Finding{}, &ParseError{Msg: fmt.Sprintf("review response contained no JSON (starts with %q)", truncate(trimmed, 40))}
	}

	var findings []Finding
	if err := json.Unmarshal([]byte(extracted), &findings); err != nil {
		// Check if it's already a ParseError (e.g. missing required field from Finding.UnmarshalJSON).
		// Note: severity parse errors arrive as fmt.Errorf wrappers, not *ParseError,
		// so they fall through to the generic wrap below.
		if pe, ok := err.(*ParseError); ok {
			return []Finding{}, pe
		}
		return []Finding{}, &ParseError{Msg: "failed to parse findings JSON: " + err.Error()}
	}

	// Ensure non-nil empty slice
	if findings == nil {
		findings = []Finding{}
	}

	return findings, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
