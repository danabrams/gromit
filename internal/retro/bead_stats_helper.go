package retro

import (
	"context"
	"fmt"
	"os"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

// enrichBeadStats populates Status, CloseReason, and Comments fields on BeadStats
// by calling bd show for each bead. Errors are logged as warnings and do not stop enrichment.
func (r *Retro) enrichBeadStats(_ context.Context, beadStats map[string]logger.BeadStats) {
	if r == nil || beadStats == nil {
		return
	}

	client, err := bead.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create bead client for enrichment: %v\n", err)
		return
	}

	var skippedMissingBeads int
	for beadID, stats := range beadStats {
		// Get full bead details (may fail for beads deleted since the log was written)
		b, err := client.Show(beadID)
		if err != nil {
			skippedMissingBeads++
			continue
		}

		// Populate status and close reason
		stats.Status = b.Status
		stats.CloseReason = b.CloseReason

		// Get comments
		comments, err := client.GetComments(beadID)
		if err != nil {
			stats.Comments = []string{}
		} else {
			// Extract comment text into a slice
			commentTexts := make([]string, len(comments))
			for i, c := range comments {
				commentTexts[i] = c.Text
			}
			stats.Comments = commentTexts
		}

		// Update the map entry
		beadStats[beadID] = stats
	}
	if skippedMissingBeads > 0 {
		fmt.Fprintf(os.Stderr, "Note: skipped %d bead(s) no longer in bd\n", skippedMissingBeads)
	}
}
