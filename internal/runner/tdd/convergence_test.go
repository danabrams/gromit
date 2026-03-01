package tdd

import "testing"

func TestEvaluateCycleConvergenceStable(t *testing.T) {
    snapshots := []CycleSnapshot{
        {CycleNumber: 1, Remaining: []string{"implement feature"}},
    }
    if got := EvaluateCycleConvergence(snapshots); got != ConvergenceStable {
        t.Fatalf("expected stable convergence result, got %v", got)
    }
}
