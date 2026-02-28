package tui

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/queue"
)

// RenderQueueView builds a textual representation of the queue based on the store snapshot.
func RenderQueueView(store *Store, focusedPanel int) string {
	if store == nil {
		return ""
	}

	store.mu.RLock()
	snapshot := store.Queue.Snapshot
	if snapshot == nil {
		store.mu.RUnlock()
		return ""
	}
	ready := append([]*bead.Bead{}, snapshot.Ready...)
	blocked := append([]*bead.Bead{}, snapshot.Blocked...)
	stuck := append([]*bead.Bead{}, snapshot.Stuck...)
	all := append([]*bead.Bead{}, snapshot.All...)
	store.mu.RUnlock()

	var b strings.Builder

	b.WriteString("=== Ready Beads")
	b.WriteString(panelFocus(focusedPanel, 0))
	b.WriteString(" ===\n")
	if len(ready) == 0 {
		b.WriteString("No ready beads\n")
	} else {
		for i, bead := range ready {
			fmt.Fprintf(&b, "%d. %s\n", i+1, bead.Title)
		}
	}

	b.WriteString("\n=== Blocked Beads")
	b.WriteString(panelFocus(focusedPanel, 1))
	b.WriteString(" ===\n")
	if len(blocked) == 0 {
		b.WriteString("No blocked beads\n")
	} else {
		for _, bead := range blocked {
			reason := queue.GetReason(bead, all)
			fmt.Fprintf(&b, "- %s (%s)\n", bead.Title, reason)
		}
	}

	b.WriteString("\n=== Stuck Beads ===\n")
	if len(stuck) == 0 {
		b.WriteString("No stuck beads\n")
	} else {
		for _, bead := range stuck {
			fmt.Fprintf(&b, "- %s\n", bead.Title)
		}
	}

	return b.String()
}
