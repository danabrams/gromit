package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagesdesc "github.com/danabrams/gromit/internal/v2/stages/gate"
)

// Stage evaluates bead readiness before build work begins.
type Stage struct {
	name    string
	tracker tasktracker.TaskTracker
}

// New constructs a gate stage backed by the provided configuration and tracker.
func New(cfg *config.Config, tracker tasktracker.TaskTracker) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	return &Stage{name: stagesdesc.Describe(cfg), tracker: tracker}, nil
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

	if hasBlockingDependencies(b) {
		return &stagepkg.Result{Decision: stagepkg.DecisionBlock}, nil
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func isClosed(status string) bool {
	return strings.EqualFold(status, tracker.StatusClosed)
}

func hasBlockingDependencies(b *tasktracker.Bead) bool {
	if b == nil {
		return false
	}
	return len(b.BlockedBy) > 0
}
