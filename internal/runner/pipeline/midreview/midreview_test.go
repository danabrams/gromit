package midreview_test

import (
    "context"
    "io"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/pipeline"
    "github.com/danabrams/gromit/internal/runner/pipeline/midreview"
)

func TestStage_RunSkipsWhenDisabled(t *testing.T) {
    t.Parallel()

    stage := midreview.NewStage(nil, nil, nil, io.Discard)

    cfg := &config.Config{}
    cfg.MidBuildReview.Enabled = false

    out, err := stage.Run(context.Background(), pipeline.Input{Config: cfg})
    if err != nil {
        t.Fatalf("Run() error = %v, want nil", err)
    }
    if out.Decision != pipeline.Proceed {
        t.Fatalf("Decision = %v, want Proceed", out.Decision)
    }
    if len(out.MidBuildReviewFindings) != 0 {
        t.Fatalf("Findings = %v, want none", out.MidBuildReviewFindings)
    }
}
