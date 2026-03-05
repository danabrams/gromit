package presenter

import (
    "context"
    "strings"
    "testing"

    "github.com/danabrams/gromit/internal/v2/presentation"
    v2review "github.com/danabrams/gromit/internal/v2/review"
)

func TestGitHubPresenterCreatesPR(t *testing.T) {
    t.Parallel()

    runner := &spyCommandRunner{}
    presenter := NewGitHubPresenter(runner)

    summary := presentation.PresentationSummary{
        SpecName:          "spec-success",
        SpecBranch:        "gromit/spec-spec-success",
        IntegrationBranch: "main",
        Success:           true,
        AcceptanceResults: []presentation.AcceptanceResult{
            {
                Title:       "Acceptance tests",
                Description: "All green",
            },
        },
        OutOfScopeFindings: []v2review.Finding{},
    }

    if err := presenter.PresentSummary(context.Background(), "spec-success", summary); err != nil {
        t.Fatalf("present summary: %v", err)
    }

    if runner.name != "gh" {
        t.Fatalf("unexpected command %q", runner.name)
    }
    if !assertArgMatches(runner.args, "--head", summary.SpecBranch) {
        t.Fatalf("head flag missing or incorrect: %v", runner.args)
    }
    if !assertArgMatches(runner.args, "--base", summary.IntegrationBranch) {
        t.Fatalf("base flag missing or incorrect: %v", runner.args)
    }
    if !assertArgMatches(runner.args, "--title", summary.SpecName) {
        t.Fatalf("title flag missing or incorrect: %v", runner.args)
    }
    if len(runner.args) < 1 || !strings.Contains(runner.args[len(runner.args)-1], "Acceptance tests") {
        t.Fatalf("body missing acceptance details: %v", runner.args)
    }
}

func assertArgMatches(args []string, flag, value string) bool {
    for i := 0; i+1 < len(args); i++ {
        if args[i] == flag && args[i+1] == value {
            return true
        }
    }
    return false
}

type spyCommandRunner struct {
    name string
    args []string
}

func (s *spyCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
    s.name = name
    s.args = append([]string(nil), args...)
    return "", nil
}
