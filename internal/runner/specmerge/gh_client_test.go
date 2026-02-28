package specmerge_test

import (
	"context"
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
