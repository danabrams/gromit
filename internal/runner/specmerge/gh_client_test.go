package specmerge_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestGhCLIClient_CreatePRCommandAndJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeGHRunner{
		stdout: `{
            "number": 101,
            "title": "Add spec",
            "state": "OPEN",
            "isDraft": false,
            "createdAt": "2026-01-15T10:00:00Z",
            "updatedAt": "2026-01-15T10:00:00Z",
            "url": "https://github.com/octocat/hello-world/pull/101"
        }`,
	}

	client := specmerge.NewGhCLIClient(runner)

	ctx := context.Background()
	ref, err := client.CreatePR(ctx, "Add spec changes", "Spec merge body", "spec/branch", "main")
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}

	if ref.Number != 101 {
		t.Fatalf("ref.Number = %d, want 101", ref.Number)
	}
	if ref.Owner != "octocat" {
		t.Fatalf("ref.Owner = %q, want octocat", ref.Owner)
	}
	if ref.Repo != "hello-world" {
		t.Fatalf("ref.Repo = %q, want hello-world", ref.Repo)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("runner.Run called %d times, want 1", len(runner.calls))
	}

	gotArgs := runner.calls[0].args
	wantArgs := []string{
		"pr",
		"create",
		"--json",
		"number,title,state,isDraft,createdAt,updatedAt,url",
		"--title",
		"Add spec changes",
		"--body",
		"Spec merge body",
		"--head",
		"spec/branch",
		"--base",
		"main",
	}

	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("gh command args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestGhCLIClient_GetPRCommandAndJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeGHRunner{
		stdout: `{
			"number": 202,
			"title": "Update spec",
			"state": "OPEN",
			"isDraft": true,
			"createdAt": "2026-02-01T12:00:00Z",
			"updatedAt": "2026-02-02T12:00:00Z"
		}`,
	}

	client := specmerge.NewGhCLIClient(runner)
	ctx := context.Background()
	ref := specmerge.PRRef{Owner: "octocat", Repo: "hello-world", Number: 202}

	status, err := client.GetPR(ctx, ref)
	if err != nil {
		t.Fatalf("GetPR returned error: %v", err)
	}

	if status.Number != 202 {
		t.Fatalf("status.Number = %d, want 202", status.Number)
	}
	if status.Title != "Update spec" {
		t.Fatalf("status.Title = %q, want Update spec", status.Title)
	}
	if status.State != "OPEN" {
		t.Fatalf("status.State = %q, want OPEN", status.State)
	}
	if !status.IsDraft {
		t.Fatal("status.IsDraft should be true")
	}

	if len(runner.calls) != 1 {
		t.Fatalf("runner.Run called %d times, want 1", len(runner.calls))
	}

	gotArgs := runner.calls[0].args
	wantArgs := []string{
		"pr",
		"view",
		"202",
		"--repo",
		"octocat/hello-world",
		"--json",
		"number,title,state,isDraft,createdAt,updatedAt",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("gh command args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestGhCLIClient_ListChecksCommandAndJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeGHRunner{
		stdout: `[
			{
				"name": "ci/test",
				"state": "completed",
				"bucket": "pass",
				"link": "https://example.com/checks/1"
			},
			{
				"name": "ci/lint",
				"state": "completed",
				"bucket": "fail",
				"link": "https://example.com/checks/2"
			}
		]`,
	}

	client := specmerge.NewGhCLIClient(runner)
	ctx := context.Background()
	ref := specmerge.PRRef{Owner: "octocat", Repo: "hello-world", Number: 303}

	checks, err := client.ListChecks(ctx, ref)
	if err != nil {
		t.Fatalf("ListChecks returned error: %v", err)
	}

	if len(checks) != 2 {
		t.Fatalf("ListChecks returned %d checks, want 2", len(checks))
	}
	if checks[0].Name != "ci/test" || checks[0].Status != "completed" || checks[0].Conclusion != "success" {
		t.Fatalf("first check parsed incorrectly: %+v", checks[0])
	}
	if checks[1].Conclusion != "failure" {
		t.Fatalf("second check conclusion = %q, want failure", checks[1].Conclusion)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("runner.Run called %d times, want 1", len(runner.calls))
	}

	gotArgs := runner.calls[0].args
	wantArgs := []string{
		"pr",
		"checks",
		"303",
		"--repo",
		"octocat/hello-world",
		"--json",
		"name,state,bucket,link",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("gh command args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestGhCLIClient_PostReviewCommandAndJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeGHRunner{
		stdout: `{"id": 1, "state": "APPROVED"}`,
	}

	client := specmerge.NewGhCLIClient(runner)
	ctx := context.Background()
	ref := specmerge.PRRef{Owner: "octocat", Repo: "hello-world", Number: 404}
	payload := specmerge.ReviewPayload{
		Event: "APPROVE",
		Body:  "LGTM",
		Comments: []specmerge.ReviewComment{
			{Path: "file.txt", Line: 10, Body: "Looks good"},
		},
	}

	if err := client.PostReview(ctx, ref, payload); err != nil {
		t.Fatalf("PostReview returned error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("runner.Run called %d times, want 1", len(runner.calls))
	}

	type reviewJSONComment struct {
		Path string `json:"path"`
		Line int    `json:"line"`
		Body string `json:"body"`
	}
	wantComments, _ := json.Marshal([]reviewJSONComment{
		{Path: "file.txt", Line: 10, Body: "Looks good"},
	})
	gotArgs := runner.calls[0].args
	wantArgs := []string{
		"api",
		"-X",
		"POST",
		"/repos/octocat/hello-world/pulls/404/reviews",
		"-F",
		"event=APPROVE",
		"-F",
		"body=LGTM",
		"-F",
		"comments=" + string(wantComments),
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("gh command args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestGhCLIClient_PostCommentCommandAndJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeGHRunner{
		stdout: `{"id": 2}`,
	}

	client := specmerge.NewGhCLIClient(runner)
	ctx := context.Background()
	ref := specmerge.PRRef{Owner: "octocat", Repo: "hello-world", Number: 505}

	if err := client.PostComment(ctx, ref, "Nice work!"); err != nil {
		t.Fatalf("PostComment returned error: %v", err)
	}

	gotArgs := runner.calls[0].args
	wantArgs := []string{
		"api",
		"-X",
		"POST",
		"/repos/octocat/hello-world/issues/505/comments",
		"-F",
		"body=Nice work!",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("gh command args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestGhCLIClient_RequestReviewersCommandAndJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeGHRunner{
		stdout: `{"requested_reviewers": [{"login": "reviewer1"}, {"login": "reviewer2"}]}`,
	}

	client := specmerge.NewGhCLIClient(runner)
	ctx := context.Background()
	ref := specmerge.PRRef{Owner: "octocat", Repo: "hello-world", Number: 606}

	if err := client.RequestReviewers(ctx, ref, []string{"reviewer1", "reviewer2"}); err != nil {
		t.Fatalf("RequestReviewers returned error: %v", err)
	}

	gotArgs := runner.calls[0].args
	wantArgs := []string{
		"api",
		"-X",
		"POST",
		"/repos/octocat/hello-world/pulls/606/requested_reviewers",
		"-F",
		"reviewers=[\"reviewer1\",\"reviewer2\"]",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("gh command args = %v, want %v", gotArgs, wantArgs)
	}
}

// fakeGHRunner captures gh CLI commands for testing.
type fakeGHRunner struct {
	calls  []ghCall
	stdout string
	err    error
}

type ghCall struct {
	args []string
}

func (f *fakeGHRunner) Run(ctx context.Context, args ...string) (string, error) {
	callArgs := append([]string(nil), args...)
	f.calls = append(f.calls, ghCall{args: callArgs})
	return f.stdout, f.err
}
