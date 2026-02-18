package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/scope"
)

func (r *Runner) maybeRunSpecGate(ctx context.Context, st *runLoopState, specName string) error {
	if r == nil || st == nil || r.cfg == nil {
		return nil
	}
	if specName == "" {
		return nil
	}
	if !r.cfg.SpecGate.IsEnabled() || !r.cfg.SpecGate.IsAutoTrigger() {
		return nil
	}
	if r.specGate == nil || r.beads == nil {
		return nil
	}

	specsDir := r.cfg.Paths.Specs
	if err := scope.ValidateSpec(specsDir, specName); err != nil {
		return err
	}
	labels := scope.ResolveSpec(specName)
	if len(labels) == 0 {
		return fmt.Errorf("no label found for spec %q", specName)
	}

	beads, err := r.beads.ListWithLabel(labels[0])
	if err != nil {
		return err
	}
	for _, b := range beads {
		if b != nil && strings.EqualFold(b.Status, "open") {
			return nil
		}
	}

	return nil
}
