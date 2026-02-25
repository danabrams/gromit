package state

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// stubReviewTag replaces the review-tag git helpers with fakes that return the
// given output/error. It restores the originals on test cleanup and returns a
// capture struct so callers can inspect the last command invoked.
type reviewTagCapture struct {
	name string
	args []string
	dir  string
}

func stubReviewTag(t *testing.T, output []byte, outputErr error) *reviewTagCapture {
	t.Helper()

	origCommandFn := reviewTagCommandFn
	origOutputFn := reviewTagOutputFn
	t.Cleanup(func() {
		reviewTagCommandFn = origCommandFn
		reviewTagOutputFn = origOutputFn
	})

	capture := &reviewTagCapture{}
	reviewTagCommandFn = func(name string, arg ...string) *exec.Cmd {
		capture.name = name
		capture.args = append([]string{}, arg...)
		return exec.Command("echo", "stub")
	}
	reviewTagOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
		capture.dir = cmd.Dir
		return output, outputErr
	}

	return capture
}

// stubReviewTagSequence replaces the review-tag git helpers with a fake that
// returns different output for each successive call, cycling through the
// provided responses.
type reviewTagResponse struct {
	output []byte
	err    error
}

func stubReviewTagSequence(t *testing.T, responses []reviewTagResponse) *[]reviewTagCapture {
	t.Helper()

	origCommandFn := reviewTagCommandFn
	origOutputFn := reviewTagOutputFn
	t.Cleanup(func() {
		reviewTagCommandFn = origCommandFn
		reviewTagOutputFn = origOutputFn
	})

	var captures []reviewTagCapture
	callIdx := 0

	reviewTagCommandFn = func(name string, arg ...string) *exec.Cmd {
		captures = append(captures, reviewTagCapture{name: name, args: append([]string{}, arg...)})
		return exec.Command("echo", "stub")
	}
	reviewTagOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
		idx := callIdx
		callIdx++
		if idx < len(captures) {
			captures[idx].dir = cmd.Dir
		}
		if idx < len(responses) {
			return responses[idx].output, responses[idx].err
		}
		return nil, errors.New("unexpected call")
	}

	return &captures
}

func TestCreateReviewTag_CreatesTagWithPrefix(t *testing.T) {
	capture := stubReviewTag(t, []byte(""), nil)

	if err := CreateReviewTag("abc123"); err != nil {
		t.Fatalf("CreateReviewTag() error = %v", err)
	}

	if capture.name != "git" {
		t.Fatalf("expected git command, got %q", capture.name)
	}
	if len(capture.args) < 3 || capture.args[0] != "tag" {
		t.Fatalf("expected git tag command, got args %v", capture.args)
	}
	if !strings.HasPrefix(capture.args[1], reviewTagPrefix) {
		t.Fatalf("tag %q should have prefix %q", capture.args[1], reviewTagPrefix)
	}
	if capture.args[2] != "abc123" {
		t.Fatalf("expected commit abc123, got %q", capture.args[2])
	}
}

func TestCreateReviewTag_ReturnsErrorOnFailure(t *testing.T) {
	stubReviewTag(t, nil, errors.New("git tag failed"))

	err := CreateReviewTag("abc123")
	if err == nil {
		t.Fatal("expected error from CreateReviewTag")
	}
	if !strings.Contains(err.Error(), "creating review tag") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCreateReviewTagInRepo_SetsCommandDir(t *testing.T) {
	capture := stubReviewTag(t, []byte(""), nil)

	if err := CreateReviewTagInRepo("/tmp/repo", "abc123"); err != nil {
		t.Fatalf("CreateReviewTagInRepo() error = %v", err)
	}
	if capture.dir != "/tmp/repo" {
		t.Fatalf("git command dir = %q, want %q", capture.dir, "/tmp/repo")
	}
}

func TestCreateReviewTagInRepo_RejectsEmptyCommit(t *testing.T) {
	err := CreateReviewTagInRepo("/tmp/repo", "   ")
	if err == nil {
		t.Fatal("expected error from CreateReviewTagInRepo with empty commit")
	}
	if !strings.Contains(err.Error(), "commit cannot be empty") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLatestReviewTagCommit_ReturnsLatestTagCommit(t *testing.T) {
	stubReviewTagSequence(t, []reviewTagResponse{
		{output: []byte("gromit/interactive-review/2026-02-25T10-00-00\ngromit/interactive-review/2026-02-24T09-00-00\n"), err: nil},
		{output: []byte("deadbeef123\n"), err: nil},
	})

	commit, err := LatestReviewTagCommit()
	if err != nil {
		t.Fatalf("LatestReviewTagCommit() error = %v", err)
	}
	if commit != "deadbeef123" {
		t.Fatalf("LatestReviewTagCommit() = %q, want %q", commit, "deadbeef123")
	}
}

func TestLatestReviewTagCommit_ReturnsEmptyWhenNoTags(t *testing.T) {
	stubReviewTag(t, []byte(""), nil)

	commit, err := LatestReviewTagCommit()
	if err != nil {
		t.Fatalf("LatestReviewTagCommit() error = %v", err)
	}
	if commit != "" {
		t.Fatalf("LatestReviewTagCommit() = %q, want empty", commit)
	}
}

func TestLatestReviewTagCommit_ReturnsErrorOnListFailure(t *testing.T) {
	stubReviewTag(t, nil, errors.New("git tag -l failed"))

	_, err := LatestReviewTagCommit()
	if err == nil {
		t.Fatal("expected error from LatestReviewTagCommit")
	}
	if !strings.Contains(err.Error(), "listing review tags") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLatestReviewTagCommit_ReturnsErrorOnRevListFailure(t *testing.T) {
	stubReviewTagSequence(t, []reviewTagResponse{
		{output: []byte("gromit/interactive-review/2026-02-25T10-00-00\n"), err: nil},
		{output: nil, err: errors.New("rev-list failed")},
	})

	_, err := LatestReviewTagCommit()
	if err == nil {
		t.Fatal("expected error from LatestReviewTagCommit")
	}
	if !strings.Contains(err.Error(), "resolving tag") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLatestReviewTagCommit_MultipleTagsReturnsFirst(t *testing.T) {
	captures := stubReviewTagSequence(t, []reviewTagResponse{
		{output: []byte("gromit/interactive-review/2026-02-25T12-00-00\ngromit/interactive-review/2026-02-25T10-00-00\ngromit/interactive-review/2026-02-24T09-00-00\n"), err: nil},
		{output: []byte("latest-commit-hash\n"), err: nil},
	})

	commit, err := LatestReviewTagCommit()
	if err != nil {
		t.Fatalf("LatestReviewTagCommit() error = %v", err)
	}
	if commit != "latest-commit-hash" {
		t.Fatalf("LatestReviewTagCommit() = %q, want %q", commit, "latest-commit-hash")
	}

	// Verify rev-list was called with the first (most recent) tag.
	if len(*captures) < 2 {
		t.Fatalf("expected 2 git calls, got %d", len(*captures))
	}
	revListArgs := (*captures)[1].args
	if len(revListArgs) < 3 || revListArgs[2] != "gromit/interactive-review/2026-02-25T12-00-00" {
		t.Fatalf("rev-list should target first tag, got args %v", revListArgs)
	}
}

func TestLatestReviewTagCommitInRepo_SetsCommandDir(t *testing.T) {
	captures := stubReviewTagSequence(t, []reviewTagResponse{
		{output: []byte("gromit/interactive-review/2026-02-25T12-00-00\n"), err: nil},
		{output: []byte("latest-commit-hash\n"), err: nil},
	})

	_, err := LatestReviewTagCommitInRepo("/tmp/repo")
	if err != nil {
		t.Fatalf("LatestReviewTagCommitInRepo() error = %v", err)
	}
	if len(*captures) != 2 {
		t.Fatalf("expected 2 git calls, got %d", len(*captures))
	}
	if (*captures)[0].dir != "/tmp/repo" || (*captures)[1].dir != "/tmp/repo" {
		t.Fatalf("expected both git calls to run in /tmp/repo, got dirs %q and %q", (*captures)[0].dir, (*captures)[1].dir)
	}
}
