package testutil

import "context"

// FakeAgent records calls and returns configured responses.
type FakeAgent struct {
	Response string
	Err      error
	Calls    []string
}

func (a *FakeAgent) Run(_ context.Context, prompt string) (string, error) {
	a.Calls = append(a.Calls, prompt)
	return a.Response, a.Err
}
