package build

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagesdesc "github.com/danabrams/gromit/internal/v2/stages/build"
)

// PromptFragments groups the fragments used per methodology.
type PromptFragments struct {
	Standard string
	TDD      string
	Refactor string
}

// Stage executes the build phase using the provided LLM provider.
type Stage struct {
	name      string
	cfg       *config.Config
	llm       llm.LLMProvider
	base      string
	project   string
	fragments PromptFragments
	output    io.Writer
	events.EmitterMixin
}

// New constructs a build stage.
func New(cfg *config.Config, provider llm.LLMProvider, base, project string, fragments PromptFragments, output io.Writer) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if output == nil {
		output = io.Discard
	}
	return &Stage{
		name:      stagesdesc.Describe(cfg),
		cfg:       cfg,
		llm:       provider,
		base:      base,
		project:   project,
		fragments: fragments,
		output:    output,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the stage name.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run currently returns a not implemented error.
func (s *Stage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	return nil, fmt.Errorf("build stage not implemented")
}

// WithEmitter attaches an emitter for future event propagation.
func (s *Stage) WithEmitter(emitter *events.Emitter) *Stage {
	s.EmitterMixin.SetEmitter(emitter)
	return s
}
