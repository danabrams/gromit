package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

// Stage evaluates bead readiness before build work begins.
type Stage struct {
	name    string
	tracker trackertypes.TaskTracker
}

// New constructs a gate stage backed by the provided configuration and tracker.
func New(cfg *config.Config, tracker trackertypes.TaskTracker) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	return &Stage{name: stagedesc.Describe("gate", cfg), tracker: tracker}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the gate stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run inspects the bead status and dependencies to choose a decision path.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	beadID := strings.TrimSpace(req.Bead.ID)
	if beadID == "" {
		return nil, fmt.Errorf("bead ID required")
	}

	b, err := s.tracker.ShowBead(ctx, beadID)
	if err != nil {
		return nil, fmt.Errorf("gate: show bead: %w", err)
	}
	if b == nil {
		return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
	}

	if isClosed(b.Status) {
		return &stagepkg.Result{Decision: stagepkg.DecisionSkip}, nil
	}

	if pending, err := s.hasPendingDependencies(ctx, b); err != nil {
		return nil, err
	} else if pending {
		return &stagepkg.Result{Decision: stagepkg.DecisionBlock}, nil
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func isClosed(status string) bool {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		return false
	}
	if strings.EqualFold(normalized, tracker.StatusClosed) {
		return true
	}
	return strings.EqualFold(normalized, "completed")
}

func (s *Stage) hasPendingDependencies(ctx context.Context, b *trackertypes.Bead) (bool, error) {
	if b == nil {
		return false, nil
	}

	for _, depID := range dependencyIDs(b) {
		if trimmed := strings.TrimSpace(depID); trimmed != "" {
			dep, err := s.tracker.ShowBead(ctx, trimmed)
			if err != nil {
				return false, fmt.Errorf("gate: show dependency %s: %w", trimmed, err)
			}
			if dep == nil || !isClosed(dep.Status) {
				return true, nil
			}
		}
	}

	return false, nil
}

func dependencyIDs(b *trackertypes.Bead) []string {
	if b == nil {
		return nil
	}
	deps := make([]string, 0, len(b.DependsOn)+len(b.BlockedBy))
	deps = append(deps, b.BlockedBy...)
	deps = append(deps, b.DependsOn...)
	return deps
}
