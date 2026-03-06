package accept

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/config"
)

func TestNewRejectsNilLLM(t *testing.T) {
    t.Parallel()

    _, err := New(&config.Config{}, noopGit{}, nil, "", "", "")
    if err == nil {
        t.Fatal("expected New to fail when the LLM provider is nil")
    }
}

type noopGit struct{}

func (noopGit) Checkout(context.Context, string) (string, error) { return "", nil }
func (noopGit) Diff(context.Context, string) (string, error)     { return "", nil }
