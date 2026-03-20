package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

// SatisfactionDiffer provides a diff from the branch base for satisfaction checks.
type SatisfactionDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// Stage evaluates bead readiness before build work begins.
type Stage struct {
	name    string
	tracker trackertypes.TaskTracker
	llm     llmtypes.LLMProvider // optional
	git     SatisfactionDiffer   // optional
}

// Option configures optional Stage behavior.
type Option func(*Stage)

// WithSatisfactionCheck enables pre-build satisfaction checking using the
// provided LLM and git differ.
func WithSatisfactionCheck(llm llmtypes.LLMProvider, git SatisfactionDiffer) Option {
	return func(s *Stage) {
		s.llm = llm
		s.git = git
	}
}

// New constructs a gate stage backed by the provided configuration and tracker.
func New(cfg *config.Config, tracker trackertypes.TaskTracker, opts ...Option) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	s := &Stage{name: stagedesc.Describe("gate", cfg), tracker: tracker}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
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

	if satisfied, err := s.trySatisfactionCheck(ctx, req); err != nil {
		return nil, fmt.Errorf("gate: satisfaction check: %w", err)
	} else if satisfied {
		if _, err := s.tracker.CloseBead(ctx, trackertypes.TaskTrackerCloseBeadRequest{BeadID: beadID}); err != nil {
			return nil, fmt.Errorf("gate: close satisfied bead %s: %w", beadID, err)
		}
		return &stagepkg.Result{Decision: stagepkg.DecisionSkip}, nil
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

func (s *Stage) trySatisfactionCheck(ctx context.Context, req *stagepkg.Request) (bool, error) {
	if s.llm == nil || s.git == nil {
		return false, nil
	}
	gen := generation.Current(req.Bead.Labels)
	tier := satisfactionTier(gen)
	if tier == "" {
		return false, nil
	}
	if isStructuralBead(req.Bead.Title, req.Bead.Description) {
		return false, nil
	}
	worktree := strings.TrimSpace(req.Worktree)
	if worktree == "" {
		return false, nil
	}
	diff, err := s.git.DiffFromBase(ctx, worktree)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(diff) == "" {
		return false, nil
	}
	criteria := extractCriteria(req.Bead.Description)
	return checkSatisfaction(ctx, s.llm, tier, diff, req.Bead.ID, criteria)
}

func extractCriteria(description string) []string {
	var criteria []string
	inCriteria := false
	for _, line := range strings.Split(description, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), "acceptance criteria") {
			inCriteria = true
			continue
		}
		if inCriteria {
			if strings.HasPrefix(trimmed, "- ") {
				criteria = append(criteria, strings.TrimPrefix(trimmed, "- "))
			} else if trimmed == "" {
				continue
			} else if strings.HasPrefix(trimmed, "#") {
				break
			}
		}
	}
	return criteria
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
