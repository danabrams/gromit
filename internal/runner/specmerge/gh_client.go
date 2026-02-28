package specmerge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
)

const (
	ghCreateFields = "number,title,state,isDraft,createdAt,updatedAt,url"
	ghViewFields   = "number,title,state,isDraft,createdAt,updatedAt"
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
	return nil, fmt.Errorf("ListChecks not implemented")
}

func (c *ghClient) PostReview(ctx context.Context, ref PRRef, payload ReviewPayload) error {
	return fmt.Errorf("PostReview not implemented")
}

func (c *ghClient) PostComment(ctx context.Context, ref PRRef, body string) error {
	return fmt.Errorf("PostComment not implemented")
}

func (c *ghClient) RequestReviewers(ctx context.Context, ref PRRef, reviewers []string) error {
	return fmt.Errorf("RequestReviewers not implemented")
}

func (c *ghClient) MergePR(ctx context.Context, ref PRRef, commitMessage string) error {
	return fmt.Errorf("MergePR not implemented")
}

func (c *ghClient) run(ctx context.Context, args ...string) (string, error) {
	return c.runner.Run(ctx, args...)
}

type defaultGHRunner struct{}

func (r *defaultGHRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
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
