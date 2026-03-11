package specloop

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestSpecLoop_RunsStagesInOrder(t *testing.T) {
	var order []string
	stages := []Stage{
		&recordStage{name: "init", order: &order},
		&recordStage{name: "compile", order: &order},
		&recordStage{name: "plan", order: &order},
		&recordStage{name: "finalize", order: &order},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"init", "compile", "plan", "finalize"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("want %v, got %v", want, order)
	}
}

func TestSpecLoop_StageError_SetsBlockedAndRunsEvidence(t *testing.T) {
	evidenceRan := false
	stages := []Stage{
		&recordStage{name: "init", order: new([]string)},
		&errorStage{name: "plan", err: fmt.Errorf("infra failure")},
		&recordStage{name: "execute", order: new([]string)},
		&callbackStage{name: "evidence", fn: func() { evidenceRan = true }},
		&recordStage{name: "finalize", order: new([]string)},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("SpecLoop should not propagate stage errors: %v", err)
	}
	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("want blocked, got %s", rs.Status)
	}
	if !evidenceRan {
		t.Fatal("evidence stage should still run on error")
	}
}

type recordStage struct {
	name  string
	order *[]string
}

func (s *recordStage) Name() string { return s.name }
func (s *recordStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	*s.order = append(*s.order, s.name)
	return NextAction{Kind: Continue}, nil
}

type errorStage struct {
	name string
	err  error
}

func (s *errorStage) Name() string { return s.name }
func (s *errorStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	return NextAction{}, s.err
}

type callbackStage struct {
	name string
	fn   func()
}

func (s *callbackStage) Name() string { return s.name }
func (s *callbackStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	s.fn()
	return NextAction{Kind: Continue}, nil
}

type countStage struct {
	name   string
	counts map[string]int
}

func (s *countStage) Name() string { return s.name }
func (s *countStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	s.counts[s.name]++
	return NextAction{Kind: Continue}, nil
}

type actionStage struct {
	name     string
	actionFn func() NextAction
}

func (s *actionStage) Name() string { return s.name }
func (s *actionStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	return s.actionFn(), nil
}

func TestSpecLoop_ReplanFromLoopsBack(t *testing.T) {
	callCounts := map[string]int{}
	validate := &actionStage{name: "validate", actionFn: func() NextAction {
		callCounts["validate"]++
		if callCounts["validate"] == 1 {
			return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"lint fail"}}}
		}
		return NextAction{Kind: Continue}
	}}
	stages := []Stage{
		&countStage{name: "init", counts: callCounts},
		&countStage{name: "plan", counts: callCounts},
		&countStage{name: "execute", counts: callCounts},
		validate,
		&countStage{name: "finalize", counts: callCounts},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 3, ReplanStage: "plan"})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if callCounts["plan"] != 2 {
		t.Fatalf("plan should run twice, got %d", callCounts["plan"])
	}
	if callCounts["finalize"] != 1 {
		t.Fatal("finalize should run once after second pass succeeds")
	}
	if callCounts["init"] != 1 {
		t.Fatalf("init should run once (only on cycle 1), got %d", callCounts["init"])
	}
}
