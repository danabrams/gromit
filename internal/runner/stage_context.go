package runner

import "github.com/danabrams/gromit/internal/specflow"

// StageContext captures the spec-flow metadata for a run scoped to a specific spec.
type StageContext struct {
	SpecName   string
	Stage      specflow.Stage
	FreshStart bool
	Manager    *specflow.Manager
}

// IsSpecRun reports whether the context is tied to a spec run.
func (s *StageContext) IsSpecRun() bool {
	return s != nil && s.SpecName != ""
}
