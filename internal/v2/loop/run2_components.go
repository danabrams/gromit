package loop

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

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
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
)

// Run2LoopEmitter exposes the subset of event emitter behavior needed by Run2.
type Run2LoopEmitter interface {
	Close()
}

// Run2LoopComponents groups the stages and bead loop used by the Run2 command.
type Run2LoopComponents struct {
	PlanStage             stagepkg.Stage
	PresentStage          stagepkg.Stage
	PresentSummaryContext *present.SummaryContext
	DecomposeStage        stagepkg.Stage
	BeadLoop              *BeadLoop
	Emitter               Run2LoopEmitter
}

// NewRun2LoopComponents builds the stages and bead loop that power the Run2 command.
func NewRun2LoopComponents(cfg *config.Config, adapters adapter.AdapterSet, taskTracker tasktracker.TaskTracker, provider llm.LLMProvider, legacyEmitter *events.Emitter, output io.Writer) (*Run2LoopComponents, error) {
	typedEmitter := event.NewEmitter()
	cleanup := func() {
		typedEmitter.Close()
	}

	planStage, err := planstage.New(cfg, provider, "", "", "")
	if err != nil {
		cleanup()
		return nil, err
	}
	summaryCtx := &present.SummaryContext{}
	presentStage, err := present.New(cfg, adapters.Presenter, summaryCtx)
	if err != nil {
		cleanup()
		return nil, err
	}

	decomposeStage, err := decomposestage.New(cfg, provider, taskTracker)
	if err != nil {
		cleanup()
		return nil, err
	}

	gateStage, err := gatestage.New(cfg, taskTracker)
	if err != nil {
		cleanup()
		return nil, err
	}

	buildStage, err := buildstage.New(cfg, provider, "", "", buildstage.PromptFragments{}, output)
	if err != nil {
		cleanup()
		return nil, err
	}

	validateStage, err := stagevalidate.New(cfg, NewCommandValidationRunner("."))
	if err != nil {
		cleanup()
		return nil, err
	}

	reviewStage, err := reviewstage.New(cfg, adapters.Git, provider, taskTracker, "", "", "")
	if err != nil {
		cleanup()
		return nil, err
	}
	reviewStage = reviewStage.WithEmitter(legacyEmitter)

	epilogueStage, err := epiloguestage.New(cfg, taskTracker)
	if err != nil {
		cleanup()
		return nil, err
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
		return nil, err
	}

	return &Run2LoopComponents{
		PlanStage:             planStage,
		PresentStage:          presentStage,
		PresentSummaryContext: summaryCtx,
		DecomposeStage:        decomposeStage,
		BeadLoop:              beadLoop,
		Emitter:               typedEmitter,
	}, nil
}

type noopValidationRunner struct{}

func (noopValidationRunner) Run(ctx context.Context, command string) error {
	return nil
}

// CommandValidationRunner executes shell commands and returns errors on failure.
type CommandValidationRunner struct {
	workDir string
}

// NewCommandValidationRunner creates a validation runner that executes shell commands.
func NewCommandValidationRunner(workDir string) *CommandValidationRunner {
	return &CommandValidationRunner{workDir: workDir}
}

// Run executes the command and returns an error if it fails.
func (c *CommandValidationRunner) Run(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = c.workDir
	cmd.Stdin = bytes.NewReader(nil)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("validation command failed: %w", err)
	}

	return nil
}
