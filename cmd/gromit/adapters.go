package main

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/pipeline"
)

// claudeClientAdapter adapts claude.Client to pipeline.ClaudeClient interface.
type claudeClientAdapter struct {
	Client *claude.Client
}

func (a *claudeClientAdapter) Run(prompt string, model string) (*pipeline.ClaudeRunResult, error) {
	// Use a long timeout context since the pipeline doesn't expose timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := a.Client.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}
	return &pipeline.ClaudeRunResult{
		Success:  result.Success,
		ExitCode: result.ExitCode,
		Output:   result.Output,
	}, nil
}

// beadClientAdapter adapts bead.Client to pipeline.BeadClient interface.
type beadClientAdapter struct {
	Client *bead.Client
}

func (a *beadClientAdapter) Ready() (*pipeline.BeadInfo, error) {
	b, err := a.Client.Ready()
	if err != nil {
		return nil, err
	}
	return &pipeline.BeadInfo{
		ID:       b.ID,
		Title:    b.Title,
		Priority: b.Priority,
		Labels:   b.Labels,
	}, nil
}

func (a *beadClientAdapter) Show(id string) (*pipeline.BeadInfo, error) {
	b, err := a.Client.Show(id)
	if err != nil {
		return nil, err
	}
	return &pipeline.BeadInfo{
		ID:       b.ID,
		Title:    b.Title,
		Priority: b.Priority,
		Labels:   b.Labels,
	}, nil
}

func (a *beadClientAdapter) Create(title string, priority int, labels []string, outputs []string) (*pipeline.BeadInfo, error) {
	b, err := a.Client.Create(title, priority, labels, outputs)
	if err != nil {
		return nil, err
	}
	return &pipeline.BeadInfo{
		ID:       b.ID,
		Title:    b.Title,
		Priority: b.Priority,
		Labels:   b.Labels,
	}, nil
}

func (a *beadClientAdapter) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*pipeline.BeadInfo, error) {
	b, err := a.Client.CreateWithDepsAndDescription(title, priority, labels, criteria, deps, desc)
	if err != nil {
		return nil, err
	}
	return &pipeline.BeadInfo{
		ID:       b.ID,
		Title:    b.Title,
		Priority: b.Priority,
		Labels:   b.Labels,
	}, nil
}

func (a *beadClientAdapter) Close(id string) error {
	return a.Client.Close(id)
}
