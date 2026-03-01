//go:build acceptance

package acceptance_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/readiness"
)

func TestReadinessGateBlocksUnlabeledBead(t *testing.T) {
	t.Parallel()

	result := runReadinessGateAcceptanceTest(t, readinessGateAcceptanceOptions{
		Bead: &bead.Bead{ID: "unlabeled-1", Title: "Unlabeled bead"},
		Assessment: readiness.Assessment{
			Status: readiness.StatusNotReady,
			Reason: "criteria_missing",
		},
	})

	if result.BuildInvoked {
		t.Fatalf("build stage ran despite readiness block")
	}
	if result.GateBlockReason != "criteria_missing" {
		t.Fatalf("GateBlockReason = %q, want %q", result.GateBlockReason, "criteria_missing")
	}
}
