package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
)

// claudeClientAdapter adapts claude.Client to pipeline invocation interfaces.
type claudeClientAdapter struct {
	Client  *claude.Client
	Timeout time.Duration
}

var _ pipeline.LLMClient = (*claudeClientAdapter)(nil)
var _ pipeline.ReviewInvoker = (*claudeClientAdapter)(nil)

func (a *claudeClientAdapter) Run(prompt string, model string) (*pipeline.LLMRunResult, error) {
	// Timeout is configured by the caller when constructing the adapter.
	ctx, cancel := context.WithTimeout(context.Background(), a.Timeout)
	defer cancel()

	result, err := a.Client.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}
	return toLLMRunResult(result.Success, result.ExitCode, result.Output), nil
}

type routerSelector interface {
	Select(phase string, tier string) (provider.Provider, string)
	MarkUnavailable(name string)
}

// llmRouterClientAdapter adapts provider router invocation to pipeline.ReviewInvoker.
type llmRouterClientAdapter struct {
	Router  routerSelector
	Timeout time.Duration
	Phase   string
}

var _ pipeline.LLMClient = (*llmRouterClientAdapter)(nil)
var _ pipeline.ReviewInvoker = (*llmRouterClientAdapter)(nil)

func (a *llmRouterClientAdapter) Run(prompt string, model string) (*pipeline.LLMRunResult, error) {
	if a == nil || a.Router == nil {
		return nil, fmt.Errorf("provider router adapter is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.Timeout)
	defer cancel()

	tier := provider.TierFromLegacyModel(model)
	phase := resolveProviderReviewPhase(a.Phase)

	selectedProvider, _ := a.Router.Select(phase, tier)
	if selectedProvider == nil {
		return nil, fmt.Errorf("no providers available for phase %q and tier %q", phase, tier)
	}

	result, err := selectedProvider.Run(ctx, prompt, tier)
	if err != nil && selectedProvider.IsUsageLimitError(result, err) {
		a.Router.MarkUnavailable(selectedProvider.Name())
		selectedProvider, _ = a.Router.Select(phase, tier)
		if selectedProvider != nil {
			result, err = selectedProvider.Run(ctx, prompt, tier)
		}
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("provider returned nil result")
	}

	return toLLMRunResult(result.Success, result.ExitCode, result.Output), nil
}

// retroRouterAdapter adapts provider routing to retro.ProviderRunner.
type retroRouterAdapter struct {
	Router  routerSelector
	Timeout time.Duration
	Phase   string
}

var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)

func (a *retroRouterAdapter) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if a == nil || a.Router == nil {
		return nil, fmt.Errorf("provider router adapter is nil")
	}

	phase := resolveProviderReviewPhase(a.Phase)
	selectedProvider, _ := a.Router.Select(phase, tier)
	if selectedProvider == nil {
		return nil, fmt.Errorf("no providers available for phase %q and tier %q", phase, tier)
	}

	result, err := selectedProvider.Run(ctx, prompt, tier)
	if selectedProvider.IsUsageLimitError(result, err) {
		a.Router.MarkUnavailable(selectedProvider.Name())
		selectedProvider, _ = a.Router.Select(phase, tier)
		if selectedProvider == nil {
			return nil, fmt.Errorf("no providers available for phase %q and tier %q", phase, tier)
		}
		result, err = selectedProvider.Run(ctx, prompt, tier)
	}
	return result, err
}

func (a *retroRouterAdapter) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a == nil || a.Router == nil {
		return nil, fmt.Errorf("provider router adapter is nil")
	}

	phase := resolveProviderReviewPhase(a.Phase)
	selectedProvider, _ := a.Router.Select(phase, tier)
	if selectedProvider == nil {
		return nil, fmt.Errorf("no providers available for phase %q and tier %q", phase, tier)
	}

	result, err := selectedProvider.StreamRun(ctx, prompt, tier, output, handler, onToolCall)
	if selectedProvider.IsUsageLimitError(result, err) {
		a.Router.MarkUnavailable(selectedProvider.Name())
		selectedProvider, _ = a.Router.Select(phase, tier)
		if selectedProvider == nil {
			return nil, fmt.Errorf("no providers available for phase %q and tier %q", phase, tier)
		}
		result, err = selectedProvider.StreamRun(ctx, prompt, tier, output, handler, onToolCall)
	}
	return result, err
}

// cmdAgentResolver adapts agent.Resolve to pipeline.AgentResolver.
type cmdAgentResolver struct {
	cfg *config.Config
}

var _ pipeline.AgentResolver = (*cmdAgentResolver)(nil)

func (r *cmdAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return agent.Resolve(r.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)
}

func resolveCommandAgent(cfg *config.Config, phase, flagOverride string, choosePicker bool) (agent.Agent, error) {
	resolvedAgent, err := (&cmdAgentResolver{cfg: cfg}).Resolve(phase, flagOverride, choosePicker)
	if err != nil {
		return nil, err
	}

	return commandAgentFromResolved(resolvedAgent)
}

func commandAgentFromResolved(resolvedAgent pipeline.Agent) (agent.Agent, error) {
	selectedAgent, ok := resolvedAgent.(agent.Agent)
	if !ok {
		return nil, fmt.Errorf("resolved agent does not implement agent.Agent")
	}

	return selectedAgent, nil
}

func resolveProviderReviewPhase(phase string) string {
	if phase == "" {
		return reviewSessionCommand
	}
	return phase
}

func toLLMRunResult(success bool, exitCode int, output string) *pipeline.LLMRunResult {
	return &pipeline.LLMRunResult{
		Success:  success,
		ExitCode: exitCode,
		Output:   output,
	}
}

// beadClientAdapter adapts bead.Client to pipeline.BeadClient interface.
type beadClientAdapter struct {
	Client *bead.Client
}

var _ pipeline.BeadClient = (*beadClientAdapter)(nil)

// toBeadInfo converts a bead.Bead to pipeline.BeadInfo.
func toBeadInfo(b *bead.Bead) *pipeline.BeadInfo {
	return &pipeline.BeadInfo{
		ID:       b.ID,
		Title:    b.Title,
		Priority: b.Priority,
		Labels:   b.Labels,
	}
}

func (a *beadClientAdapter) Ready() (*pipeline.BeadInfo, error) {
	b, err := a.Client.Ready()
	if err != nil {
		return nil, err
	}
	return toBeadInfo(b), nil
}

func (a *beadClientAdapter) Show(id string) (*pipeline.BeadInfo, error) {
	b, err := a.Client.Show(id)
	if err != nil {
		return nil, err
	}
	return toBeadInfo(b), nil
}

func (a *beadClientAdapter) Create(title string, priority int, labels []string, outputs []string) (*pipeline.BeadInfo, error) {
	b, err := a.Client.Create(title, priority, labels, outputs)
	if err != nil {
		return nil, err
	}
	return toBeadInfo(b), nil
}

func (a *beadClientAdapter) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*pipeline.BeadInfo, error) {
	b, err := a.Client.CreateWithDepsAndDescription(title, priority, labels, criteria, deps, desc)
	if err != nil {
		return nil, err
	}
	return toBeadInfo(b), nil
}

func (a *beadClientAdapter) Close(id string) error {
	return a.Client.Close(id)
}
