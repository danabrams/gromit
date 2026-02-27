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

	var b strings.Builder
	b.WriteString("=== Progress Panel")
	b.WriteString(panelFocus(focusedPanel, 0))
	b.WriteString(" ===\n")
	if progress := store.Dashboard.RunProgress; progress != nil {
		fmt.Fprintf(&b, "Iteration %d/%d (status=%s)\n", progress.CurrentIteration, progress.MaxIterations, progress.Status)
		fmt.Fprintf(&b, "Percent complete: %d%%\n", progress.IterationPercent)
	} else {
		b.WriteString("Iteration data unavailable\n")
	}
	b.WriteString("\n")

	ready, blocked, stuck := queueDepthCounts(store)
	fmt.Fprintf(&b, "Queue depth: ready=%d blocked=%d stuck=%d\n", ready, blocked, stuck)
	b.WriteString("\n")

	b.WriteString("Recent completions:\n")
	for _, completion := range store.Dashboard.RecentCompletions {
		fmt.Fprintf(&b, "[%s] %s\n", completion.Status, completion.BeadTitle)
	}
	b.WriteString("\n")

	healthDesc := "unknown"
	if hi := store.Dashboard.HealthIndicator; hi != nil {
		if hi.IsHealthy {
			healthDesc = "healthy"
		} else {
			healthDesc = "unhealthy"
		}
		fmt.Fprintf(&b, "Health: %s\n", healthDesc)
		if hi.LastEventType != "" {
			fmt.Fprintf(&b, "Last event: %s\n", hi.LastEventType)
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
