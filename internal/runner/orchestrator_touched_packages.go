package runner

import (
	"context"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

func mergeTouchedPackages(existing, incoming []string) []string {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	combined := append([]string(nil), existing...)
	combined = append(combined, incoming...)
	return normalizeTouchedPackages(combined)
}

func (o *Orchestrator) maybeTriggerSpecMerge(ctx context.Context, b *bead.Bead) {
	if o.cfg.SpecMergeController == nil || o.cfg.Config == nil {
		return
	}
	if o.cfg.Config.Methodology.Granularity != config.MethodologyGranularitySpec {
		return
	}
	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" {
		return
	}

	complete, err := o.cfg.SpecMergeController.IsSpecComplete(ctx, specName)
	if err != nil {
		o.logWarning("Warning: could not check spec completion for %q: %v", specName, err)
		return
	}
	if !complete {
		return
	}
	if err := o.cfg.SpecMergeController.Trigger(ctx, specName); err != nil {
		o.logWarning("Warning: spec merge pipeline trigger for %q failed: %v", specName, err)
		return
	}
	o.logInfo("Spec %q ready for human review", specName)
}

func normalizeTouchedPackages(touchedPackages []string) []string {
	uniqueTouched := make([]string, 0, len(touchedPackages))
	seen := make(map[string]struct{}, len(touchedPackages))

	for _, pkg := range touchedPackages {
		trimmed := strings.TrimSpace(pkg)
		normalized := strings.Trim(strings.TrimPrefix(trimmed, "./"), "/")
		if trimmed == "." || normalized == "." {
			normalized = "."
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		uniqueTouched = append(uniqueTouched, normalized)
	}

	return uniqueTouched
}
