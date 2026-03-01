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

func TestEvaluateCycleConvergenceDeadlock(t *testing.T) {
    snapshots := []CycleSnapshot{
        {CycleNumber: 1, Remaining: []string{"req"}},
        {CycleNumber: 2, Remaining: []string{"req"}},
    }
    if got := EvaluateCycleConvergence(snapshots); got != ConvergenceDeadlock {
        t.Fatalf("expected deadlock detection, got %v", got)
    }
}
