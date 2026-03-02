package runner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/specflow"
)

func (o *Orchestrator) runPreImplementationHook(ctx context.Context) error {
	if o == nil || o.cfg.StageContext == nil || o.cfg.StageContext.Stage != specflow.StageAcceptanceTests {
		return nil
	}
	if o.cfg.PreImplementationHook == nil {
		return fmt.Errorf("pre-implementation hook required for %s stage", specflow.StageAcceptanceTests)
	}
	if o.preImplementationHookRan {
		return nil
	}
	if err := o.cfg.PreImplementationHook(ctx); err != nil {
		return err
	}
	o.preImplementationHookRan = true
	stageCtx := o.cfg.StageContext
	if stageCtx.Manager != nil && stageCtx.SpecName != "" {
		if err := stageCtx.Manager.Advance(ctx, stageCtx.SpecName, specflow.StageImplementation); err != nil {
			return fmt.Errorf("advancing spec stage to implementation: %w", err)
		}
	}
	stageCtx.Stage = specflow.StageImplementation
	return nil
}
