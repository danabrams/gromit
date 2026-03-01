package specmerge_test

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/provider"
    "github.com/danabrams/gromit/internal/review"
    "github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestTriggerFlow_Run_AllStagesInvoked(t *testing.T) {
    t.Parallel()
    ctx := context.Background()

    var seen []string
    newStage := func(name string) specmerge.FlowStage {
        return specmerge.FlowStage{
            Name: name,
            Runner: func(ctx context.Context, specName, diff string) (*review.ReviewResult, *provider.Result, error) {
                seen = append(seen, name)
                return &review.ReviewResult{Passed: true}, nil, nil
            },
        }
    }

    flow := &specmerge.TriggerFlow{
        Stages: []specmerge.FlowStage{
            newStage("stage-one"),
            newStage("stage-two"),
        },
        DiffProvider: specmerge.DiffProviderFunc(func(ctx context.Context, specName string) (string, error) {
            if specName != "payments" {
                t.Fatalf("unexpected spec %q", specName)
            }
            return "diff --git", nil
        }),
    }

    result, err := flow.Run(ctx, "payments")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(result.StageResults) != 2 {
        t.Fatalf("stage count = %d, want 2", len(result.StageResults))
    }
    if len(seen) != 2 || seen[0] != "stage-one" || seen[1] != "stage-two" {
        t.Fatalf("stages ran = %v", seen)
    }
}

func TestTriggerFlow_Run_StopsAfterFailure(t *testing.T) {
    t.Parallel()
    ctx := context.Background()

    var seen []string
    flow := &specmerge.TriggerFlow{
        Stages: []specmerge.FlowStage{
            {
                Name: "first",
                Runner: func(ctx context.Context, specName, diff string) (*review.ReviewResult, *provider.Result, error) {
                    seen = append(seen, "first")
                    return &review.ReviewResult{Passed: true}, nil, nil
                },
            },
            {
                Name: "second",
                Runner: func(ctx context.Context, specName, diff string) (*review.ReviewResult, *provider.Result, error) {
                    seen = append(seen, "second")
                    return &review.ReviewResult{Passed: false, Summary: "needs work"}, nil, nil
                },
            },
            {
                Name: "third",
                Runner: func(ctx context.Context, specName, diff string) (*review.ReviewResult, *specmerge.ProviderResult, error) {
                    seen = append(seen, "third")
                    return &review.ReviewResult{Passed: true}, nil, nil
                },
            },
        },
        DiffProvider: specmerge.DiffProviderFunc(func(ctx context.Context, specName string) (string, error) {
            return "diff", nil
        }),
    }

    _, err := flow.Run(ctx, "spec")
    if err == nil {
        t.Fatal("expected failure error")
    }
    if len(seen) != 2 {
        t.Fatalf("stages ran = %v", seen)
    }
    if seen[1] != "second" {
        t.Fatalf("unexpected failing stage: %v", seen)
    }
}
