package runner

import "context"

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
	if r.specGate == nil {
		return nil
	}
	return nil
}
