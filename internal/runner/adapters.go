package runner

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
)

// routerAdapter wraps *provider.Router to satisfy execution.Router.
// The adapter narrows the return type from provider.Provider to execution.Provider.
type routerAdapter struct {
	r *provider.Router
}

func (a *routerAdapter) Select(phase, tier string) (execution.Provider, string) {
	p, model := a.r.Select(phase, tier)
	if p == nil {
		return nil, ""
	}
	return p, model
}

func (a *routerAdapter) MarkUnavailable(name string) {
	a.r.MarkUnavailable(name)
}

func (a *routerAdapter) RecordOutcome(providerName, failureCategory string) {
	a.r.RecordOutcome(providerName, failureCategory)
}

// makeStallTimeoutFn creates a StallTimeoutFunc that looks up per-model stall
// timeouts from the config. Used to configure the invoker's heartbeat.
func makeStallTimeoutFn(cfg *config.Config) execution.StallTimeoutFunc {
	if cfg == nil {
		return nil
	}
	return func(model string) (time.Duration, time.Duration) {
		_, st, sta, _ := cfg.Claude.TimeoutsForModel(model)
		return time.Duration(st) * time.Second, time.Duration(sta) * time.Second
	}
}

// successLearningRouterAdapter wraps *provider.Router to satisfy escalation.SuccessLearningRouter.
type successLearningRouterAdapter struct {
	r *provider.Router
}

func (a *successLearningRouterAdapter) Select(phase, tier string) (escalation.SuccessLearningProvider, string) {
	p, model := a.r.Select(phase, tier)
	if p == nil {
		return nil, ""
	}
	return &successLearningProviderAdapter{p: p}, model
}

// successLearningProviderAdapter wraps provider.Provider to satisfy escalation.SuccessLearningProvider.
type successLearningProviderAdapter struct {
	p provider.Provider
}

func (a *successLearningProviderAdapter) Run(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
	result, err := a.p.Run(ctx, prompt, tier)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &successLearningResultAdapter{r: result}, nil
}

// successLearningResultAdapter wraps *provider.Result to satisfy escalation.SuccessLearningResult.
type successLearningResultAdapter struct {
	r *provider.Result
}

func (a *successLearningResultAdapter) IsSuccess() bool   { return a.r.Success }
func (a *successLearningResultAdapter) GetOutput() string { return a.r.Output }
