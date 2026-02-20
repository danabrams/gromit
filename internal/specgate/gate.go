package specgate

import (
	"context"
	"fmt"
)

const llmInvocationCriterion = "LLM invocation"

// RunTestsFn runs the test suite and returns its output.
type RunTestsFn func(ctx context.Context) (string, error)

// InvokeLLMFn calls the LLM with a model and prompt, returning the raw response.
type InvokeLLMFn func(ctx context.Context, model, prompt string) ([]byte, error)

// RenderPromptFn renders the gate evaluation prompt from its inputs.
type RenderPromptFn func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error)

// GetDiffFn returns the current git diff.
type GetDiffFn func(ctx context.Context) (string, error)

// Gate evaluates acceptance criteria against current code state using an LLM.
type Gate struct {
	RunTests     RunTestsFn
	InvokeLLM    InvokeLLMFn
	RenderPrompt RenderPromptFn
	GetDiff      GetDiffFn
	Model        string
	// MaxCycles is part of gate orchestration config and consumed by callers.
	MaxCycles int
}

// Run evaluates the given acceptance criteria for specName, returning a GateVerdict.
func (g *Gate) Run(ctx context.Context, specName string, acceptanceCriteria []string) (*GateVerdict, error) {
	if err := g.validate(); err != nil {
		return nil, err
	}

	testOutput, err := g.RunTests(ctx)
	if err != nil {
		return nil, err
	}

	diff, err := g.GetDiff(ctx)
	if err != nil {
		return nil, err
	}

	prompt, err := g.RenderPrompt(ctx, specName, testOutput, diff, acceptanceCriteria)
	if err != nil {
		return nil, err
	}

	rawVerdict, err := g.InvokeLLM(ctx, g.Model, prompt)
	if err != nil {
		return g.llmInvocationFailureVerdict(err), nil
	}

	return ParseVerdict(rawVerdict)
}

func (g *Gate) validate() error {
	if g == nil {
		return fmt.Errorf("gate is nil")
	}
	if g.RunTests == nil {
		return fmt.Errorf("run tests dependency is nil")
	}
	if g.GetDiff == nil {
		return fmt.Errorf("get diff dependency is nil")
	}
	if g.RenderPrompt == nil {
		return fmt.Errorf("render prompt dependency is nil")
	}
	if g.InvokeLLM == nil {
		return fmt.Errorf("invoke llm dependency is nil")
	}
	return nil
}

func (g *Gate) llmInvocationFailureVerdict(err error) *GateVerdict {
	return &GateVerdict{
		Passed: false,
		Results: []CriterionResult{
			{
				Criterion: llmInvocationCriterion,
				Passed:    false,
				Evidence:  fmt.Sprintf("model %q invocation failed: %v", g.Model, err),
			},
		},
	}
}
