package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/logger"
)

func (o *Orchestrator) mergeGlobalStats() {
	if o.cfg.GlobalStatsPath == "" {
		return
	}
	var runID string
	if o.cfg.GetRunID != nil {
		runID = o.cfg.GetRunID()
	}
	if o.cfg.LogsDir == "" || runID == "" {
		return // Nothing to merge
	}
	runStats, err := logger.ReadRunModelStats(o.cfg.LogsDir, runID)
	if err != nil {
		o.logWarning("Warning: could not read run stats for global merge: %v", err)
		return
	}
	if err := logger.UpdateGlobalStats(o.cfg.GlobalStatsPath, runStats); err != nil {
		o.logWarning("Warning: could not update global stats: %v", err)
	}
}

func (o *Orchestrator) assertEfficiencyCompleteness(totalIterations int) error {
	if o.cfg.LogsDir == "" {
		return nil
	}
	var runID string
	if o.cfg.GetRunID != nil {
		runID = o.cfg.GetRunID()
	}
	if runID == "" {
		return nil
	}

	result, diags := logger.AssertEfficiencyCompleteness(o.cfg.LogsDir, runID)

	baseDiags := make([]string, 0, len(diags)+1)
	baseDiags = append(baseDiags, diags...)
	if result.ErrorMessage != "" {
		baseDiags = append(baseDiags, result.ErrorMessage)
	}
	joinDiagnostics := func(extras ...string) string {
		parts := make([]string, 0, len(baseDiags)+len(extras))
		parts = append(parts, baseDiags...)
		for _, extra := range extras {
			if extra == "" {
				continue
			}
			parts = append(parts, extra)
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "\n  ")
	}

	if totalIterations > 0 && result.TotalIterations < totalIterations {
		missingRows := totalIterations - result.TotalIterations
		missingMsg := fmt.Sprintf("%d iteration log row(s) missing for run %s (found %d/%d entries)", missingRows, runID, result.TotalIterations, totalIterations)
		diagMsg := joinDiagnostics(missingMsg)
		if diagMsg == "" {
			diagMsg = missingMsg
		}
		return fmt.Errorf("efficiency data completeness assertion failed: %d/%d iteration logs missing\nDiagnostics:\n  %s", missingRows, totalIterations, diagMsg)
	}
	if result.TotalIterations > 0 && !result.IsComplete {
		diagMsg := joinDiagnostics()
		if diagMsg == "" {
			diagMsg = "no diagnostics available"
		}
		return fmt.Errorf("efficiency data completeness assertion failed: %d/%d iterations missing efficiency data\nDiagnostics:\n  %s", result.MissingDataCount, result.TotalIterations, diagMsg)
	}

	return nil
}

func (o *Orchestrator) checkControlLimitAlerts() {
	const firstPassSuccessThreshold = 0.80
	const minimumWindowSize = 30

	if o.cfg.LogsDir == "" || o.cfg.StateSaver == nil {
		return
	}

	trend, err := logger.ReadProcessTrend(o.cfg.LogsDir)
	if err != nil {
		o.logWarning("Warning: could not read process trend for control limit check: %v", err)
		return
	}

	if trend == nil {
		return
	}

	if trend.LatestWindow.FirstPassSuccess < firstPassSuccessThreshold && trend.WindowSize >= minimumWindowSize {
		o.logWarning("Warning: first-pass success rate %.1f%% is below control limit of %.0f%% (window: %d iterations); review recent decomposition rule changes",
			trend.LatestWindow.FirstPassSuccess*100, firstPassSuccessThreshold*100, trend.WindowSize)

		if setter, ok := o.cfg.StateSaver.(interface{ SetControlLimitAlertTriggered(bool) }); ok {
			setter.SetControlLimitAlertTriggered(true)
		}
	}
}

func (o *Orchestrator) runAutoTriage(ctx context.Context) {
	if o.cfg.AutoTriageService == nil {
		return
	}
	if err := o.cfg.AutoTriageService.EvaluateAndTriage(ctx); err != nil {
		o.logWarning("Warning: auto-triage evaluation failed: %v", err)
	}
}
