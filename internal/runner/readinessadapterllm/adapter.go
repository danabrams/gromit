package readinessadapterllm

import (
    "github.com/danabrams/gromit/internal/prompt"
    "github.com/danabrams/gromit/internal/provider"
)

type readinessPromptRenderer interface {
    RenderReadiness(ctx *prompt.ReadinessContext) (string, error)
}

type readinessRouter interface {
    Select(phase, tier string) (provider.Provider, string)
}

type readinessAdapterWithLLM struct {
    renderer readinessPromptRenderer
    router   readinessRouter
}

func NewReadinessAdapterWithLLM(renderer readinessPromptRenderer, router readinessRouter) *readinessAdapterWithLLM {
    if renderer == nil || router == nil {
        return nil
    }
    return &readinessAdapterWithLLM{
        renderer: renderer,
        router:   router,
    }
}
