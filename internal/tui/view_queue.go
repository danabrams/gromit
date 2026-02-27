package tui

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/queue"
)

// RenderQueueView builds a textual representation of the queue based on the store snapshot.
func RenderQueueView(store *Store, focusedPanel int) string {
	if store == nil || store.Queue.Snapshot == nil {
		return ""
	}

	var b strings.Builder
	snapshot := store.Queue.Snapshot

	b.WriteString("=== Ready Beads")
	b.WriteString(panelFocus(focusedPanel, 0))
	b.WriteString(" ===\n")
	if len(snapshot.Ready) == 0 {
		b.WriteString("No ready beads\n")
	} else {
		for i, bead := range snapshot.Ready {
			fmt.Fprintf(&b, "%d. %s\n", i+1, bead.Title)
		}
	}

	b.WriteString("\n=== Blocked Beads")
	b.WriteString(panelFocus(focusedPanel, 1))
	b.WriteString(" ===\n")
	if len(snapshot.Blocked) == 0 {
		b.WriteString("No blocked beads\n")
	} else {
		for _, bead := range snapshot.Blocked {
			reason := queue.GetReason(bead, snapshot.All)
			fmt.Fprintf(&b, "- %s (%s)\n", bead.Title, reason)
		}
	}

	b.WriteString("\n=== Stuck Beads ===\n")
	if len(snapshot.Stuck) == 0 {
		b.WriteString("No stuck beads\n")
	} else {
		for _, bead := range snapshot.Stuck {
			fmt.Fprintf(&b, "- %s\n", bead.Title)
		}
	}

	return b.String()
}
