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
