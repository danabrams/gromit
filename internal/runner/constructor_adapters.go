package runner

import (
	"encoding/json"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/escalation"
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
