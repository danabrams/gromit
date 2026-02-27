package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
	"github.com/danabrams/gromit/internal/tracker"
)

// Adapters in this file transform internal package types to pipeline.Deps interface types.
// The adapter pattern enables testability and loose coupling:
// - Each adapter wraps a single internal dependency (e.g., claude.Client, bead.Client)
// - Each adapter implements one or more pipeline interfaces (e.g., LLMClient, TrackerClient)
// - Adapters perform minimal type transformation and delegation to underlying dependencies
// - New adapters are wired together in adapter_deps.go by NewPipelineDeps()
//
// Adapter naming conventions:
// - General adapters use "Adapter" suffix (e.g., claudeClientAdapter, trackerClientAdapter)
// - No business logic in adapters - they are pure type-transforming bridges

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

func resolveCommandAgent(cfg *config.Config, phase, flagOverride string, choosePicker bool) (agent.Agent, error) {
	resolvedAgent, err := agent.NewResolver(cfg).Resolve(phase, flagOverride, choosePicker)
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

// trackerClientAdapter adapts tracker.Client to pipeline.TrackerClient interface.
type trackerClientAdapter struct {
	Client tracker.Client
}

var _ pipeline.TrackerClient = (*trackerClientAdapter)(nil)

// trackerItemToBeadInfo converts a tracker.Item to pipeline.BeadInfo.
func trackerItemToBeadInfo(item *tracker.Item) *pipeline.BeadInfo {
	if item == nil {
		return nil
	}

	priority := 0
	if p, ok := item.Metadata["priority"]; ok {
		fmt.Sscanf(p, "%d", &priority)
	}

	var labels []string
	if l, ok := item.Metadata["labels"]; ok {
		// Labels are stored as JSON array in metadata
		json.Unmarshal([]byte(l), &labels)
	}

	return &pipeline.BeadInfo{
		ID:       item.ID,
		Title:    item.Title,
		Priority: priority,
		Labels:   labels,
	}
}

func (a *trackerClientAdapter) Ready(ctx context.Context) (*pipeline.BeadInfo, error) {
	item, err := a.Client.Ready(ctx)
	if err != nil {
		return nil, err
	}
	return trackerItemToBeadInfo(item), nil
}

func (a *trackerClientAdapter) Show(ctx context.Context, id string) (*pipeline.BeadInfo, error) {
	item, err := a.Client.Show(ctx, id)
	if err != nil {
		return nil, err
	}
	return trackerItemToBeadInfo(item), nil
}

func (a *trackerClientAdapter) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*pipeline.BeadInfo, error) {
	req := tracker.CreateRequest{
		Title:    title,
		Metadata: make(map[string]string),
	}
	if priority > 0 {
		req.Metadata["priority"] = fmt.Sprintf("%d", priority)
	}
	if len(labels) > 0 {
		labelsJSON, _ := json.Marshal(labels)
		req.Metadata["labels"] = string(labelsJSON)
	}
	if encoded, ok := tracker.EncodeMetadataJSONList(outputs); ok {
		req.Metadata["expected_outputs"] = encoded
	}

	item, err := a.Client.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return trackerItemToBeadInfo(item), nil
}

func (a *trackerClientAdapter) CreateWithDepsAndDescription(ctx context.Context, title string, priority int, labels []string, criteria []string, deps []string, desc string) (*pipeline.BeadInfo, error) {
	req := tracker.CreateRequest{
		Title:       title,
		Description: desc,
		Metadata:    make(map[string]string),
	}
	if priority > 0 {
		req.Metadata["priority"] = fmt.Sprintf("%d", priority)
	}
	if len(labels) > 0 {
		labelsJSON, _ := json.Marshal(labels)
		req.Metadata["labels"] = string(labelsJSON)
	}
	if encoded, ok := tracker.EncodeMetadataJSONList(criteria); ok {
		req.Metadata["acceptance_criteria"] = encoded
	}
	if encoded, ok := tracker.EncodeMetadataJSONList(deps); ok {
		req.Metadata["dependencies"] = encoded
	}

	item, err := a.Client.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return trackerItemToBeadInfo(item), nil
}

func (a *trackerClientAdapter) Close(ctx context.Context, id string) error {
	return a.Client.Close(ctx, id)
}

// backlogClientAdapter adapts backlog.File to pipeline.BacklogClient interface.
type backlogClientAdapter struct {
	file *backlog.File
}

var _ pipeline.BacklogClient = (*backlogClientAdapter)(nil)

func (a *backlogClientAdapter) List() ([]*pipeline.Idea, error) {
	return a.file.List()
}

func (a *backlogClientAdapter) Get(id string) (*pipeline.Idea, error) {
	return a.file.Get(id)
}

func (a *backlogClientAdapter) Add(item *pipeline.Idea) error {
	return a.file.Add(item)
}

func (a *backlogClientAdapter) Update(id string, fn func(*pipeline.Idea)) error {
	return a.file.Update(id, fn)
}

// beadQueryClientAdapter adapts bead.Client to pipeline.BeadQueryClient interface.
type beadQueryClientAdapter struct {
	Client *bead.Client
}

var _ pipeline.BeadQueryClient = (*beadQueryClientAdapter)(nil)

func (a *beadQueryClientAdapter) CountByStatus(ctx context.Context, status string) (int, error) {
	if a == nil || a.Client == nil {
		return 0, fmt.Errorf("bead query adapter is nil")
	}
	return a.Client.CountByStatus(ctx, status)
}

func (a *beadQueryClientAdapter) ListReadyIDs(ctx context.Context) ([]string, error) {
	if a == nil || a.Client == nil {
		return nil, fmt.Errorf("bead query adapter is nil")
	}
	return a.Client.ListReadyIDs(ctx)
}

func (a *beadQueryClientAdapter) CountClosedAfter(ctx context.Context, after time.Time) (int, error) {
	if a == nil || a.Client == nil {
		return 0, fmt.Errorf("bead query adapter is nil")
	}
	return a.Client.CountClosedAfter(ctx, after)
}
