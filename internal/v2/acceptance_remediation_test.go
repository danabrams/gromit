//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRemediationCycleTriggersGapAnalysis(t *testing.T) {
	t.Parallel()

	order := []string{}
	accept := &flakyAcceptStage{order: &order}
	gap := &recordingStage{name: "gap-analysis", order: &order}
	decompose := &recordingDecomposeStage{
		order: &order,
		beads: []*bead.Bead{{ID: "missing-123"}},
	}
	beadLoop := &fakeBeadRunner{order: &order}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		GapStage:       gap,
		DecomposeStage: decompose,
		BeadRunner:     beadLoop,
		GenerationCap:  3,
	})

	if err := runner.Run(context.Background(), "spec-remediate"); err != nil {
		t.Fatalf("remediation run failed: %v", err)
	}

	want := []string{"accept", "gap-analysis", "decompose", "bead-loop", "accept"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
}

// Helpers used in remediation acceptance tests.
type flakyAcceptStage struct {
	order    *[]string
	attempts int
}

func (f *flakyAcceptStage) Name() string { return "accept" }

func (f *flakyAcceptStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	f.attempts++
	*f.order = append(*f.order, "accept")
	if f.attempts == 1 {
		return &stage.Result{Decision: stage.DecisionFail}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

var _ stage.Stage = (*flakyAcceptStage)(nil)

type recordingStage struct {
	name  string
	order *[]string
}

func (r *recordingStage) Name() string { return r.name }

func (r *recordingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	*r.order = append(*r.order, r.name)
	return nil, nil
}

var _ stage.Stage = (*recordingStage)(nil)

type recordingDecomposeStage struct {
	order *[]string
	beads []*bead.Bead
}

func (r *recordingDecomposeStage) Name() string { return "decompose" }

func (r *recordingDecomposeStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	*r.order = append(*r.order, "decompose")
	return &stage.Result{Artifacts: &DecomposeArtifacts{Beads: r.beads}}, nil
}

var _ stage.Stage = (*recordingDecomposeStage)(nil)

type fakeBeadRunner struct {
	order *[]string
	runs  int
}

func (f *fakeBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	*f.order = append(*f.order, "bead-loop")
	f.runs++
	return nil
}
