package build

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// PromptFragments groups template fragments for each build methodology.
type PromptFragments struct {
	Standard string
	TDD      string
	Refactor string
}

// Methodology represents a build methodology.
type Methodology string

const (
	MethodologyStandard Methodology = "standard"
	MethodologyTDD      Methodology = "tdd"
	MethodologyRefactor Methodology = "refactor"
)

// BuildArtifacts exposes telemetry and output returned by the build stage.
type BuildArtifacts struct {
	Model    string
	Prompt   string
	Output   string
	Tokens   int
	CostUSD  float64
	Duration time.Duration
	Success  bool
}

// Stage implements the build stage.
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

// New constructs a build stage backed by the provided dependencies.
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
		name:      stagedesc.Describe("build", cfg),
		cfg:       cfg,
		llm:       provider,
		base:      base,
		project:   project,
		fragments: fragments,
		output:    output,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical build stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the build LLM invocation, handles model escalation, and reports results.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	cfg, err := s.resolveConfig(req)
	if err != nil {
		return nil, err
	}

	methodology := s.resolveMethodology(req.Bead.Labels, cfg)
	fragment := s.fragmentFor(methodology)
	instance := buildInstanceLayer(req)
	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, fragment).Assemble()

	model := s.selectModel(req, cfg)

	resp, finalModel, invokeErr := s.invokeWithEscalation(ctx, promptText, model, cfg)
	if invokeErr != nil {
		return nil, fmt.Errorf("build: %w", invokeErr)
	}

	artifacts := &BuildArtifacts{
		Model:    finalModel,
		Prompt:   promptText,
		Output:   resp.Output,
		Tokens:   resp.Tokens,
		CostUSD:  resp.CostUSD,
		Duration: resp.Duration,
		Success:  resp.Success,
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts, Events: []event.TypedEvent{}}, nil
}

func (s *Stage) resolveConfig(req *stagepkg.Request) (*config.Config, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return cfg, nil
}

func (s *Stage) fragmentFor(methodology Methodology) string {
	switch methodology {
	case MethodologyTDD:
		if s.fragments.TDD != "" {
			return s.fragments.TDD
		}
	case MethodologyRefactor:
		if s.fragments.Refactor != "" {
			return s.fragments.Refactor
		}
	}
	return s.fragments.Standard
}

func (s *Stage) resolveMethodology(labels []string, cfg *config.Config) Methodology {
	methodology := selectMethodology(labels, cfg)
	strategy := resolveBuildStrategy(cfg, labels)

	if strings.EqualFold(strategy, "tdd") {
		return MethodologyTDD
	}
	if strings.EqualFold(strategy, "single_pass") && hasBuildStrategyLabel(labels, "build_strategy:single_pass") {
		if methodology == MethodologyTDD || methodology == MethodologyRefactor {
			return MethodologyStandard
		}
	}
	return methodology
}

func selectMethodology(labels []string, cfg *config.Config) Methodology {
	tddGlobal := cfg != nil && cfg.Methodology.TDD
	if bead.IsMethodologyActive(labels, "tdd", tddGlobal) {
		return MethodologyTDD
	}
	if bead.IsMethodologyActive(labels, "refactor", false) {
		return MethodologyRefactor
	}
	return MethodologyStandard
}

func resolveBuildStrategy(cfg *config.Config, labels []string) string {
	strategy := "single_pass"
	if cfg != nil && cfg.Methodology.BuildStrategy != "" {
		strategy = strings.TrimSpace(cfg.Methodology.BuildStrategy)
	}
	for _, label := range labels {
		switch label {
		case "build_strategy:tdd":
			strategy = "tdd"
		case "build_strategy:single_pass":
			strategy = "single_pass"
		}
	}
	return strategy
}

func hasBuildStrategyLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

func buildInstanceLayer(req *stagepkg.Request) string {
	if req == nil {
		return ""
	}
	var builder strings.Builder
	if id := strings.TrimSpace(req.Bead.ID); id != "" {
		builder.WriteString("Bead ID: ")
		builder.WriteString(id)
	}
	if req.RetryContext != nil && len(req.RetryContext.PriorFailures) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("Prior failures:")
		for _, failure := range req.RetryContext.PriorFailures {
			builder.WriteString("\n- ")
			builder.WriteString(failure)
		}
	}
	return builder.String()
}

func (s *Stage) selectModel(req *stagepkg.Request, cfg *config.Config) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Model); trimmed != "" {
			return trimmed
		}
	}
	if cfg != nil {
		if trimmed := strings.TrimSpace(cfg.Models.P1); trimmed != "" {
			return trimmed
		}
		if len(cfg.Escalation.Chain) > 0 {
			return cfg.Escalation.Chain[0]
		}
	}
	return config.ModelSonnet
}

func (s *Stage) writer() io.Writer {
	if s.output != nil {
		return s.output
	}
	return io.Discard
}

func (s *Stage) invokeWithEscalation(ctx context.Context, prompt, initialModel string, cfg *config.Config) (*llm.LLMResponse, string, error) {
	model := initialModel
	escalationCfg := cfg
	if escalationCfg == nil {
		escalationCfg = s.cfg
	}

	for {
		resp, err := s.llm.StreamInvoke(ctx, llm.StreamInvokeRequest{Prompt: prompt, Model: model, Output: s.writer()})
		if err == nil && resp != nil && resp.Success {
			return resp, model, nil
		}

		var reason error
		if err != nil {
			reason = err
		} else if resp == nil {
			reason = fmt.Errorf("provider returned nil response")
		} else if !resp.Success {
			reason = fmt.Errorf("provider reported unsuccessful result")
		}

		if escalationCfg == nil || !escalationCfg.Escalation.Enabled {
			return resp, model, reason
		}

		nextModel := escalationCfg.NextEscalationModel(model)
		if nextModel == "" {
			return resp, model, reason
		}
		model = nextModel
	}
}

func (s *Stage) buildStartEvent(req *stagepkg.Request, model string, cfg *config.Config) events.Event {
	attempt := 1
	if req != nil && req.RetryContext != nil {
		attempt += req.RetryContext.Attempt
	}
	maxAttempts := 1
	if cfg != nil && cfg.Escalation.Enabled && len(cfg.Escalation.Chain) > 0 {
		maxAttempts = len(cfg.Escalation.Chain)
	}
	return &events.BuildStartEvent{
		BeadID:      req.Bead.ID,
		Model:       model,
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
		TimeMixin:   events.TimeMixin{Time: time.Now()},
	}
}

func (s *Stage) buildCompleteEvent(req *stagepkg.Request, resp *llm.LLMResponse) events.Event {
	duration := time.Duration(0)
	tokens := 0
	cost := 0.0
	success := false
	if resp != nil {
		duration = resp.Duration
		tokens = resp.Tokens
		cost = resp.CostUSD
		success = resp.Success
	}
	return &events.BuildCompleteEvent{
		BeadID:    req.Bead.ID,
		Success:   success,
		Duration:  duration,
		Cost:      cost,
		TokensIn:  tokens,
		TokensOut: 0,
		TimeMixin: events.TimeMixin{Time: time.Now()},
	}
}

// WithEmitter attaches an emitter for downstream consumers.
func (s *Stage) WithEmitter(emitter *events.Emitter) *Stage {
	s.EmitterMixin.SetEmitter(emitter)
	return s
}
