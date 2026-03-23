package proposaltriage

import "context"

// stubLLMCompleter is a test stub for LLMCompleter
type stubLLMCompleter struct {
	response string
	err      error
}

func (s *stubLLMCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.response, nil
}
