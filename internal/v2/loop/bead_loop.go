package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/queue"
	"github.com/danabrams/gromit/internal/v2/dep"
	"github.com/danabrams/gromit/internal/v2/stage"
)

// BeadLoopConfig holds the stages required to process each bead.
type BeadLoopConfig struct {
	Gate     stage.Stage
	Build    stage.Stage
	Validate stage.Stage
	Review   stage.Stage
	Epilogue stage.Stage
}

// BeadLoop orchestrates the per-bead execution pipeline.
type BeadLoop struct {
	gate     stage.Stage
	build    stage.Stage
	validate stage.Stage
	review   stage.Stage
	epilogue stage.Stage
}

// NewBeadLoop constructs a BeadLoop tagged with the provided stages.
func NewBeadLoop(config BeadLoopConfig) (*BeadLoop, error) {
	if config.Gate == nil {
		return nil, fmt.Errorf("gate stage required")
	}
	if config.Build == nil {
		return nil, fmt.Errorf("build stage required")
	}
	if config.Validate == nil {
		return nil, fmt.Errorf("validate stage required")
	}
	if config.Review == nil {
		return nil, fmt.Errorf("review stage required")
	}
	if config.Epilogue == nil {
		return nil, fmt.Errorf("epilogue stage required")
	}
	return &BeadLoop{
		gate:     config.Gate,
		build:    config.Build,
		validate: config.Validate,
		review:   config.Review,
		epilogue: config.Epilogue,
	}, nil
}

// Run processes the provided beads through the stage pipeline.
func (b *BeadLoop) Run(ctx context.Context, beads []*bead.Bead) error {
	resolver := dep.NewResolver()
	beadMap := make(map[string]*bead.Bead, len(beads))

	for _, beadItem := range beads {
		if beadItem == nil {
			continue
		}
		id := strings.TrimSpace(beadItem.ID)
		if id == "" {
			continue
		}
		beadMap[id] = beadItem
		resolver.Add(id, collectDependencies(beadItem))
	}

	var completed []string
	for {
		next, err := resolver.Next(completed)
		if err != nil {
			return fmt.Errorf("resolve bead: %w", err)
		}
		if next == "" {
			break
		}
		beadItem, ok := beadMap[next]
		if !ok {
			return fmt.Errorf("bead %q missing from input list", next)
		}
		if err := b.processBead(ctx, beadItem); err != nil {
			return err
		}
		completed = append(completed, next)
	}
	return nil
}

func (b *BeadLoop) processBead(ctx context.Context, beadItem *bead.Bead) error {
	for _, staged := range []stage.Stage{b.gate, b.build, b.validate, b.review, b.epilogue} {
		if err := b.runStage(ctx, staged, beadItem); err != nil {
			return err
		}
	}
	return nil
}

func (b *BeadLoop) runStage(ctx context.Context, staged stage.Stage, beadItem *bead.Bead) error {
	if staged == nil {
		return nil
	}
	req := b.stageRequest(beadItem)
	res, err := staged.Run(ctx, &req)
	if err != nil {
		return fmt.Errorf("stage %s: %w", staged.Name(), err)
	}
	if res != nil && res.Decision != stage.DecisionProceed {
		return fmt.Errorf("stage %s returned %s", staged.Name(), res.Decision)
	}
	return nil
}

func (b *BeadLoop) stageRequest(beadItem *bead.Bead) stage.Request {
	labels := copyLabels(beadItem.Labels)
	return stage.Request{
		Bead: stage.BeadInfo{ID: beadItem.ID, Labels: labels},
	}
}

func copyLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	copied := make([]string, len(labels))
	copy(copied, labels)
	return copied
}

func collectDependencies(b *bead.Bead) []string {
	if b == nil {
		return nil
	}
	deps := queue.DependencyIDs(b.DependsOn)
	deps = append(deps, queue.DependencyIDs(b.Dependencies)...)
	deps = append(deps, queue.DependencyIDs(b.BlockedBy)...)
	return deps
}
