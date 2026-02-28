package tui

import (
	"fmt"
	"strings"
)

// RenderDashboardView returns the dashboard view string built from the provided store.
func RenderDashboardView(store *Store, focusedPanel int) string {
	if store == nil {
		return ""
	}

	store.mu.RLock()
	var (
		progressCopy    *RunProgress
		healthCopy      *HealthIndicator
		completionsCopy []*Completion
		ready           int
		blocked         int
		stuck           int
	)
	if progress := store.Dashboard.RunProgress; progress != nil {
		progressCopy = &RunProgress{
			CurrentIteration: progress.CurrentIteration,
			MaxIterations:    progress.MaxIterations,
			IterationPercent: progress.IterationPercent,
			Status:           progress.Status,
		}
	}
	ready, blocked, stuck = queueDepthCounts(store)
	if health := store.Dashboard.HealthIndicator; health != nil {
		healthCopy = &HealthIndicator{
			LastEventType:    health.LastEventType,
			LastEventTime:    health.LastEventTime,
			IsHealthy:        health.IsHealthy,
			HasStalledBeads:  health.HasStalledBeads,
			WarningThreshold: health.WarningThreshold,
		}
	}
	if len(store.Dashboard.RecentCompletions) > 0 {
		completionsCopy = append([]*Completion{}, store.Dashboard.RecentCompletions...)
	}
	store.mu.RUnlock()

	var b strings.Builder
	b.WriteString("=== Progress Panel")
	b.WriteString(panelFocus(focusedPanel, 0))
	b.WriteString(" ===\n")
	if progressCopy != nil {
		fmt.Fprintf(&b, "Iteration %d/%d (status=%s)\n", progressCopy.CurrentIteration, progressCopy.MaxIterations, progressCopy.Status)
		fmt.Fprintf(&b, "Percent complete: %d%%\n", progressCopy.IterationPercent)
	} else {
		b.WriteString("Iteration data unavailable\n")
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "Queue depth: ready=%d blocked=%d stuck=%d\n", ready, blocked, stuck)
	b.WriteString("\n")

	b.WriteString("Recent completions:\n")
	for _, completion := range completionsCopy {
		fmt.Fprintf(&b, "[%s] %s\n", completion.Status, completion.BeadTitle)
	}
	b.WriteString("\n")

	healthDesc := "unknown"
	if healthCopy != nil {
		if healthCopy.IsHealthy {
			healthDesc = "healthy"
		} else {
			healthDesc = "unhealthy"
		}
		fmt.Fprintf(&b, "Health: %s\n", healthDesc)
		if healthCopy.LastEventType != "" {
			fmt.Fprintf(&b, "Last event: %s\n", healthCopy.LastEventType)
		}
	}

	return b.String()
}

func queueDepthCounts(store *Store) (ready, blocked, stuck int) {
	if store == nil || store.Queue.Snapshot == nil {
		return 0, 0, 0
	}

	snapshot := store.Queue.Snapshot
	ready = len(snapshot.Ready)
	blocked = len(snapshot.Blocked)
	stuck = len(snapshot.Stuck)
	return ready, blocked, stuck
}

func panelFocus(focusedPanel, panelID int) string {
	if focusedPanel == panelID {
		return " [FOCUSED]"
	}
	return ""
}
