//go:build integration

package loop

import "testing"

func TestIntegrationImmutable_StageSequenceStructuredCommits(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-stage-sequence",
		beads: immutableBeads(
			immutableBead("bead-001", "First bead"),
		),
	})

	assertStructuredStageSequence(t, result, []immutableCommitExpectation{
		{BeadID: "spec", StageName: "present", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "accept", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "review", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "validate", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "build", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "decompose", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "plan", Iteration: 1, Decision: "proceed"},
	})
}
