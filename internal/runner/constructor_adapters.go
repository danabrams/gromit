package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func selectEscalationHandler(
	cfg *config.Config,
	analyzer escalation.FailureAnalyzer,
	beadClient escalation.BeadClient,
	decomposeFn escalation.DecomposeFn,
	createSubFn escalation.CreateSubFn,
	logFn escalation.LogFn,
	showPartialProgressFn escalation.ShowPartialProgressFn,
) interface{} {
	if cfg == nil {
		return escalation.NewHandler(cfg, analyzer, beadClient, decomposeFn, createSubFn, logFn, showPartialProgressFn)
	}

	strategy := strings.TrimSpace(cfg.Routing.Strategy)
	if strings.EqualFold(strategy, "cost_optimized") {
		maxRetries := cfg.Routing.CostOptimized.MaxRetriesBeforeDecompose
		if maxRetries <= 0 {
			maxRetries = 2
		}
		return escalation.NewDecomposeFirstHandler(
			cfg,
			analyzer,
			beadClient,
			decomposeFn,
			createSubFn,
			logFn,
			showPartialProgressFn,
			maxRetries,
		)
	}

	return escalation.NewHandler(cfg, analyzer, beadClient, decomposeFn, createSubFn, logFn, showPartialProgressFn)
}

func toJSONList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(items)
	return string(data)
}

type stuckDetectorAdapter struct {
	logsDir   string
	gromitDir string
	policy    policy.StuckPolicy
}

func (a *stuckDetectorAdapter) IsStuck(ctx context.Context, b *bead.Bead) (bool, error) {
	if a == nil || a.policy == nil || b == nil {
		return false, nil
	}

	store := newRestartPointStore(a.gromitDir)
	if err := store.load(); err != nil {
		return false, fmt.Errorf("loading restart points: %w", err)
	}

	stats, err := logger.ReadPerBeadStatsAfter(a.logsDir, store.all())
	if err != nil {
		return false, fmt.Errorf("reading bead stats: %w", err)
	}

	return a.policy.IsStuck(b, stats), nil
}

type restartPoint struct {
	RestartAt time.Time `json:"restart_at"`
	Reason    string    `json:"reason,omitempty"`
}

type restartPointStore struct {
	path   string
	points map[string]restartPoint
}

func newRestartPointStore(gromitDir string) *restartPointStore {
	return &restartPointStore{
		path:   filepath.Join(gromitDir, "restart-points.json"),
		points: map[string]restartPoint{},
	}
}

func (s *restartPointStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.points = map[string]restartPoint{}
			return nil
		}
		return err
	}
	if len(data) == 0 {
		s.points = map[string]restartPoint{}
		return nil
	}

	points := map[string]restartPoint{}
	if err := json.Unmarshal(data, &points); err != nil {
		return err
	}
	s.points = points
	return nil
}

func (s *restartPointStore) all() map[string]time.Time {
	result := make(map[string]time.Time, len(s.points))
	for id, pt := range s.points {
		if pt.RestartAt.IsZero() {
			continue
		}
		result[id] = pt.RestartAt
	}
	return result
}
