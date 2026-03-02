package specmerge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/procutil"
)

const (
	ghCreateFields = "number,title,state,isDraft,createdAt,updatedAt,url"
	ghViewFields   = "number,title,state,isDraft,createdAt,updatedAt"
	ghChecksFields = "name,state,bucket,link"
)

type ghCommandRunner interface {
	Run(context.Context, ...string) (string, error)
}

type ghClient struct {
	runner ghCommandRunner
}

// NewGhCLIClient returns a PRClient backed by the gh CLI.
func NewGhCLIClient(runner ghCommandRunner) PRClient {
	if runner == nil {
		runner = &defaultGHRunner{}
	}
	return &ghClient{runner: runner}
}

func (c *ghClient) CreatePR(ctx context.Context, title, body, head, base string) (PRRef, error) {
	args := []string{
		"pr",
		"create",
		"--json",
		ghCreateFields,
		"--title",
		title,
		"--body",
		body,
		"--head",
		head,
		"--base",
		base,
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return PRRef{}, fmt.Errorf("create pr: %w", err)
	}

	var resp struct {
		Number    int    `json:"number"`
		URL       string `json:"url"`
		Title     string `json:"title"`
		State     string `json:"state"`
		IsDraft   bool   `json:"isDraft"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return PRRef{}, fmt.Errorf("parse create pr response: %w", err)
	}

	owner, repo, err := parseRepoFromURL(resp.URL)
	if err != nil {
		return PRRef{}, fmt.Errorf("parse repo from url: %w", err)
	}

	return PRRef{Owner: owner, Repo: repo, Number: resp.Number}, nil
}

func (c *ghClient) GetPR(ctx context.Context, ref PRRef) (PRStatus, error) {
	args := []string{
		"pr",
		"view",
		strconv.Itoa(ref.Number),
		"--repo",
		fmt.Sprintf("%s/%s", ref.Owner, ref.Repo),
		"--json",
		ghViewFields,
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return PRStatus{}, fmt.Errorf("get pr: %w", err)
	}

	var resp struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		IsDraft   bool   `json:"isDraft"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return PRStatus{}, fmt.Errorf("parse get pr response: %w", err)
	}

	return PRStatus{
		Number:    resp.Number,
		Title:     resp.Title,
		State:     resp.State,
		IsDraft:   resp.IsDraft,
		CreatedAt: resp.CreatedAt,
		UpdatedAt: resp.UpdatedAt,
	}, nil
}

func (c *ghClient) ListChecks(ctx context.Context, ref PRRef) ([]CheckStatus, error) {
	args := []string{
		"pr",
		"checks",
		strconv.Itoa(ref.Number),
		"--repo",
		fmt.Sprintf("%s/%s", ref.Owner, ref.Repo),
		"--json",
		ghChecksFields,
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list checks: %w", err)
	}

	var resp []struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Bucket string `json:"bucket"`
		Link   string `json:"link"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse checks response: %w", err)
	}

	checks := make([]CheckStatus, len(resp))
	for i, check := range resp {
		checks[i] = CheckStatus{
			Name:       check.Name,
			Status:     check.State,
			Conclusion: conclusionFromBucket(check.Bucket),
			DetailsURL: check.Link,
		}
	}

	return checks, nil
}

func (c *ghClient) PostReview(ctx context.Context, ref PRRef, payload ReviewPayload) error {
	args := []string{
		"api",
		"-X",
		"POST",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", ref.Owner, ref.Repo, ref.Number),
		"-f",
		"event=" + payload.Event,
	}

	if payload.Body != "" {
		args = append(args, "-f", "body="+payload.Body)
	}

	if len(payload.Comments) > 0 {
		mapped := make([]ghReviewComment, 0, len(payload.Comments))
		for _, comment := range payload.Comments {
			mapped = append(mapped, ghReviewComment{
				Path: comment.Path,
				Line: comment.Line,
				Body: comment.Body,
			})
		}
		encoded, err := json.Marshal(mapped)
		if err != nil {
			return fmt.Errorf("marshal review comments: %w", err)
		}
		args = append(args, "-f", "comments="+string(encoded))
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("post review: %w", err)
	}

	var resp struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("parse post review response: %w", err)
	}

	return nil
}

func (c *ghClient) PostComment(ctx context.Context, ref PRRef, body string) error {
	args := []string{
		"api",
		"-X",
		"POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number),
		"-f",
		"body=" + body,
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("post comment: %w", err)
	}

	var resp struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("parse post comment response: %w", err)
	}

	return nil
}

func (c *ghClient) RequestReviewers(ctx context.Context, ref PRRef, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	encoded, err := json.Marshal(reviewers)
	if err != nil {
		return fmt.Errorf("marshal reviewers: %w", err)
	}

	args := []string{
		"api",
		"-X",
		"POST",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", ref.Owner, ref.Repo, ref.Number),
		"-f",
		"reviewers=" + string(encoded),
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("request reviewers: %w", err)
	}

	var resp struct {
		RequestedReviewers []struct {
			Login string `json:"login"`
		} `json:"requested_reviewers"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("parse request reviewers response: %w", err)
	}

	return nil
}

func (c *ghClient) MergePR(ctx context.Context, ref PRRef, commitMessage string) error {
	args := []string{
		"api",
		"-X",
		"PUT",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", ref.Owner, ref.Repo, ref.Number),
		"-f",
		"commit_message=" + commitMessage,
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("merge pr: %w", err)
	}

	var resp struct {
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("parse merge response: %w", err)
	}

	if !resp.Merged {
		return fmt.Errorf("merge pr failed: %s", resp.Message)
	}

	return nil
}

func conclusionFromBucket(bucket string) string {
	switch strings.ToLower(strings.TrimSpace(bucket)) {
	case "pass":
		return "success"
	case "fail":
		return "failure"
	case "cancel":
		return "cancelled"
	case "neutral":
		return "neutral"
	case "pending":
		return "pending"
	case "skipped":
		return "skipped"
	default:
		return bucket
	}
}

type ghReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

func (c *ghClient) run(ctx context.Context, args ...string) (string, error) {
	return c.runner.Run(ctx, args...)
}

type defaultGHRunner struct{}

var (
	defaultGHReaper            = procutil.ReapProcessTree
	ghSetProcessGroupKillFn    = procutil.SetProcessGroupKill
	ghWaitForProcessCapacityFn = procutil.WaitForProcessCapacity
)

const ghProcessCapacityWait = 1500 * time.Millisecond

func (r *defaultGHRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	ghSetProcessGroupKillFn(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if waitErr := ghWaitForProcessCapacityFn(ctx, ghProcessCapacityWait); waitErr != nil {
		return "", fmt.Errorf("waiting for process capacity: %w", waitErr)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("gh start: %w", err)
	}
	procutil.KillDescendantsOnCancel(ctx, cmd)
	defer defaultGHReaper(cmd)

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}

func parseRepoFromURL(raw string) (string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid url %q: %w", raw, err)
	}
	trimmed := strings.Trim(parsed.Path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected pr url path %q", parsed.Path)
	}
	return parts[0], parts[1], nil
}
