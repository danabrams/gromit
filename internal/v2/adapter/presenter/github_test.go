package presenter

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
)

func TestGitHubPresenterCreatesPR(t *testing.T) {
	t.Parallel()

	runner := &spyCommandRunner{prViewErr: fmt.Errorf("no PR found")}
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
	if runner.bodyContent == "" || !strings.Contains(runner.bodyContent, "Acceptance tests") {
		t.Fatalf("body missing acceptance details: %q", runner.bodyContent)
	}
}

func TestGitHubPresenterUsesBodyFile(t *testing.T) {
	t.Parallel()

	runner := &spyCommandRunner{prViewErr: fmt.Errorf("no PR found")}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-team",
		SpecBranch:        "gromit/spec-team",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
	}

	if err := presenter.PresentSummary(context.Background(), "spec-team", summary); err != nil {
		t.Fatalf("present summary: %v", err)
	}

	if runner.bodyFile == "" {
		t.Fatalf("expected --body-file arg: %v", runner.args)
	}
	if !strings.Contains(runner.bodyContent, "Acceptance tests") {
		t.Fatalf("body file missing acceptance details: %q", runner.bodyContent)
	}
	expected := presentation.RenderPRBody(summary)
	if runner.bodyContent != expected {
		t.Fatalf("unexpected body file contents\nexpected:\n%q\nactual:\n%q", expected, runner.bodyContent)
	}
}

func TestGitHubPresenterEditsPRWhenExists(t *testing.T) {
	t.Parallel()

	// prViewErr is nil — simulates an existing PR.
	runner := &spyCommandRunner{}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-existing",
		SpecBranch:        "gromit/spec-existing",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
	}

	if err := presenter.PresentSummary(context.Background(), "spec-existing", summary); err != nil {
		t.Fatalf("present summary: %v", err)
	}

	// Should have two calls: pr view, then pr edit.
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(runner.calls), runner.calls)
	}

	viewCall := runner.calls[0]
	if viewCall.name != "gh" || viewCall.args[0] != "pr" || viewCall.args[1] != "view" {
		t.Fatalf("first call should be gh pr view, got %q %v", viewCall.name, viewCall.args)
	}

	editCall := runner.calls[1]
	if editCall.name != "gh" || editCall.args[0] != "pr" || editCall.args[1] != "edit" {
		t.Fatalf("second call should be gh pr edit, got %q %v", editCall.name, editCall.args)
	}
	if !assertArgMatches(editCall.args, "--title", summary.SpecName) {
		t.Fatalf("edit call missing --title: %v", editCall.args)
	}

	// Should NOT have called pr create.
	for _, c := range runner.calls {
		if len(c.args) >= 2 && c.args[0] == "pr" && c.args[1] == "create" {
			t.Fatalf("should not call pr create when PR exists, but got: %v", c.args)
		}
	}

	// Body file should contain acceptance details.
	if runner.bodyContent == "" || !strings.Contains(runner.bodyContent, "Acceptance tests") {
		t.Fatalf("body missing acceptance details: %q", runner.bodyContent)
	}
}

func TestGitHubPresenterCreatesPRWhenNoneExists(t *testing.T) {
	t.Parallel()

	// prViewErr is set — simulates no existing PR.
	runner := &spyCommandRunner{prViewErr: fmt.Errorf("no pull requests found")}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-new",
		SpecBranch:        "gromit/spec-new",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
	}

	if err := presenter.PresentSummary(context.Background(), "spec-new", summary); err != nil {
		t.Fatalf("present summary: %v", err)
	}

	// Should have two calls: pr view (fails), then pr create.
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(runner.calls), runner.calls)
	}

	viewCall := runner.calls[0]
	if viewCall.name != "gh" || viewCall.args[0] != "pr" || viewCall.args[1] != "view" {
		t.Fatalf("first call should be gh pr view, got %q %v", viewCall.name, viewCall.args)
	}

	createCall := runner.calls[1]
	if createCall.name != "gh" || createCall.args[0] != "pr" || createCall.args[1] != "create" {
		t.Fatalf("second call should be gh pr create, got %q %v", createCall.name, createCall.args)
	}
	if !assertArgMatches(createCall.args, "--head", summary.SpecBranch) {
		t.Fatalf("create call missing --head: %v", createCall.args)
	}
	if !assertArgMatches(createCall.args, "--base", summary.IntegrationBranch) {
		t.Fatalf("create call missing --base: %v", createCall.args)
	}

	// Should NOT have called pr edit.
	for _, c := range runner.calls {
		if len(c.args) >= 2 && c.args[0] == "pr" && c.args[1] == "edit" {
			t.Fatalf("should not call pr edit when no PR exists, but got: %v", c.args)
		}
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

type spyCall struct {
	name string
	args []string
}

type spyCommandRunner struct {
	name        string
	args        []string
	bodyFile    string
	bodyContent string
	calls       []spyCall

	// prViewErr controls the error returned by "pr view" calls.
	// If non-nil, "gh pr view ..." will return this error (simulating no existing PR).
	// If nil, "gh pr view ..." succeeds (simulating an existing PR).
	prViewErr error
}

func (s *spyCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	s.name = name
	s.args = append([]string(nil), args...)
	s.calls = append(s.calls, spyCall{name: name, args: append([]string(nil), args...)})

	// If this is a "pr view" call, return the configured error.
	if len(args) >= 2 && args[0] == "pr" && args[1] == "view" {
		return "", s.prViewErr
	}

	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--body-file" {
			s.bodyFile = args[i+1]
			data, err := os.ReadFile(s.bodyFile)
			if err != nil {
				return "", err
			}
			s.bodyContent = string(data)
		}
	}
	return "", nil
}
