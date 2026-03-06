package loop

import (
	"context"
	"io"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/event"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	buildstage "github.com/danabrams/gromit/internal/v2/stage/build"
	decomposestage "github.com/danabrams/gromit/internal/v2/stage/decompose"
	epiloguestage "github.com/danabrams/gromit/internal/v2/stage/epilogue"
	gatestage "github.com/danabrams/gromit/internal/v2/stage/gate"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
)

// Run2LoopEmitter exposes the subset of event emitter behavior needed by Run2.
type Run2LoopEmitter interface {
	Close()
}

// NewRun2LoopComponents builds the stages and bead loop that power the Run2 command.
func NewRun2LoopComponents(cfg *config.Config, adapters adapter.AdapterSet, taskTracker tasktracker.TaskTracker, provider llm.LLMProvider, legacyEmitter *events.Emitter, output io.Writer) (stagepkg.Stage, *BeadLoop, Run2LoopEmitter, error) {
	typedEmitter := event.NewEmitter()
	cleanup := func() {
		typedEmitter.Close()
	}

	decomposeStage, err := decomposestage.New(cfg, provider, taskTracker)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	gateStage, err := gatestage.New(cfg, taskTracker)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	buildStage, err := buildstage.New(cfg, provider, "", "", buildstage.PromptFragments{}, output)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	validateStage, err := stagevalidate.New(cfg, &noopValidationRunner{})
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	reviewStage, err := reviewstage.New(cfg, adapters.Git, provider, taskTracker, "", "", "")
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	reviewStage = reviewStage.WithEmitter(legacyEmitter)

	epilogueStage, err := epiloguestage.New(cfg, taskTracker)
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	beadLoop, err := NewBeadLoop(BeadLoopConfig{
		Gate:          gateStage,
		Build:         buildStage,
		Validate:      validateStage,
		Review:        reviewStage,
		Epilogue:      epilogueStage,
		Emitter:       typedEmitter,
		LegacyEmitter: legacyEmitter,
	})
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}

	return decomposeStage, beadLoop, typedEmitter, nil
}

type noopValidationRunner struct{}

func (noopValidationRunner) Run(ctx context.Context, command string) error {
	return nil
}
