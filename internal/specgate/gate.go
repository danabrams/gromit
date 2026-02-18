package specgate

import "context"

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
	MaxCycles    int
}

// Run evaluates the given acceptance criteria for specName, returning a GateVerdict.
func (g *Gate) Run(ctx context.Context, specName string, acceptanceCriteria []string) (*GateVerdict, error) {
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

	raw, err := g.InvokeLLM(ctx, g.Model, prompt)
	if err != nil {
		return nil, err
	}

	return ParseVerdict(raw)
}
