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
		SpecBranch:        "gromit/spec/spec-success",
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

	// First call should be git push with --force-with-lease.
	if len(runner.calls) < 1 || runner.calls[0].name != "git" {
		t.Fatalf("first call should be git push, got %+v", runner.calls)
	}
	if !containsArg(runner.calls[0].args, "--force-with-lease") {
		t.Fatalf("push call missing --force-with-lease: %v", runner.calls[0].args)
	}

	if runner.name != "gh" {
		t.Fatalf("unexpected last command %q", runner.name)
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
		SpecBranch:        "gromit/spec/team",
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
		SpecBranch:        "gromit/spec/existing",
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

	// Should have three calls: git push, pr view, then pr edit.
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d: %+v", len(runner.calls), runner.calls)
	}

	pushCall := runner.calls[0]
	if pushCall.name != "git" || pushCall.args[0] != "push" {
		t.Fatalf("first call should be git push, got %q %v", pushCall.name, pushCall.args)
	}
	if !containsArg(pushCall.args, "--force-with-lease") {
		t.Fatalf("push call missing --force-with-lease: %v", pushCall.args)
	}

	viewCall := runner.calls[1]
	if viewCall.name != "gh" || viewCall.args[0] != "pr" || viewCall.args[1] != "view" {
		t.Fatalf("second call should be gh pr view, got %q %v", viewCall.name, viewCall.args)
	}

	editCall := runner.calls[2]
	if editCall.name != "gh" || editCall.args[0] != "pr" || editCall.args[1] != "edit" {
		t.Fatalf("third call should be gh pr edit, got %q %v", editCall.name, editCall.args)
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
		SpecBranch:        "gromit/spec/new",
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

	// Should have three calls: git push, pr view (fails), then pr create.
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d: %+v", len(runner.calls), runner.calls)
	}

	pushCall := runner.calls[0]
	if pushCall.name != "git" || pushCall.args[0] != "push" {
		t.Fatalf("first call should be git push, got %q %v", pushCall.name, pushCall.args)
	}
	if !containsArg(pushCall.args, "--force-with-lease") {
		t.Fatalf("push call missing --force-with-lease: %v", pushCall.args)
	}

	viewCall := runner.calls[1]
	if viewCall.name != "gh" || viewCall.args[0] != "pr" || viewCall.args[1] != "view" {
		t.Fatalf("second call should be gh pr view, got %q %v", viewCall.name, viewCall.args)
	}

	createCall := runner.calls[2]
	if createCall.name != "gh" || createCall.args[0] != "pr" || createCall.args[1] != "create" {
		t.Fatalf("third call should be gh pr create, got %q %v", createCall.name, createCall.args)
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

func TestPresent_PublishedURLComesFromCommandOutput(t *testing.T) {
	t.Parallel()

	// The spy runner returns a PR URL from the "gh pr view" command.
	expectedURL := "https://github.com/owner/repo/pull/42"
	runner := &spyCommandRunner{
		prViewOutput: expectedURL,
	}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-url-test",
		SpecBranch:        "gromit/spec/url-test",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
	}

	resp, err := presenter.Present(context.Background(), PresentRequest{
		SpecID:          "spec-url-test",
		Summary:         summary,
		DestinationHint: "some-hint",
	})
	if err != nil {
		t.Fatalf("Present failed: %v", err)
	}

	// PublishedURL should come from the command output, NOT from DestinationHint.
	if resp.PublishedURL == "some-hint" {
		t.Fatal("PublishedURL should not echo back the DestinationHint")
	}
	if resp.PublishedURL != expectedURL {
		t.Fatalf("PublishedURL should be %q, got %q", expectedURL, resp.PublishedURL)
	}
}

func TestPresent_PublishedURLFromCreateCommand(t *testing.T) {
	t.Parallel()

	// When no PR exists, the "gh pr create" command outputs the URL.
	expectedURL := "https://github.com/owner/repo/pull/99"
	runner := &spyCommandRunner{
		prViewErr:      fmt.Errorf("no PR found"),
		prCreateOutput: expectedURL,
	}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-create-url",
		SpecBranch:        "gromit/spec/create-url",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
	}

	resp, err := presenter.Present(context.Background(), PresentRequest{
		SpecID:          "spec-create-url",
		Summary:         summary,
		DestinationHint: "another-hint",
	})
	if err != nil {
		t.Fatalf("Present failed: %v", err)
	}

	if resp.PublishedURL != expectedURL {
		t.Fatalf("PublishedURL should be %q, got %q", expectedURL, resp.PublishedURL)
	}
}

func containsArg(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
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

	// prViewOutput is the stdout returned by successful "pr view" calls.
	prViewOutput string

	// prCreateOutput is the stdout returned by successful "pr create" calls.
	prCreateOutput string
}

func (s *spyCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	s.name = name
	s.args = append([]string(nil), args...)
	s.calls = append(s.calls, spyCall{name: name, args: append([]string(nil), args...)})

	// If this is a "pr view" call, return the configured error or output.
	if len(args) >= 2 && args[0] == "pr" && args[1] == "view" {
		return s.prViewOutput, s.prViewErr
	}

	// If this is a "pr create" call, return the configured output.
	if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
		// Read body file before returning.
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
		return s.prCreateOutput, nil
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
