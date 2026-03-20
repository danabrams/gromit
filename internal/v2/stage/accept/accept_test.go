package accept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestNewRejectsNilLLM(t *testing.T) {
	t.Parallel()

	_, err := New(&config.Config{}, &fakeGitAdapter{}, nil, "", "", "")
	if err == nil {
		t.Fatal("expected New to fail when the LLM provider is nil")
	}
}

func TestRunWritesGapAnalysisWhenCriterionFails(t *testing.T) {
	t.Parallel()

	specID := "spec-gap"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specDir, specID+".md")
	content := "# Gap spec\n\n## Acceptance Criteria\n- satisfy A\n- satisfy B\n"
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths: config.PathsConfig{
			GromitDir: ".gromit",
			Specs:     ".gromit/specs",
		},
	}

	git := &fakeGitAdapter{diff: "diff content"}
	llmProvider := &fakeLLM{
		responses: []*llm.LLMResponse{
			{Success: true, Output: `{"pass": true, "summary": "ok"}`},
			{Success: true, Output: `{"pass": false, "summary": "missing tests"}`},
		},
	}

	stageInstance, err := New(cfg, git, llmProvider, "BASE", "PROJECT", "FRAGMENT")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	stageInstance.batchDiffThreshold = -1 // force per-criterion evaluation

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID},
		Worktree: tmp,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res.Decision != stagepkg.DecisionFail {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionFail)
	}

	artifacts, ok := res.Artifacts.(*AcceptArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T, want *AcceptArtifacts", res.Artifacts)
	}
	if artifacts.GapSummary == "" {
		t.Fatalf("gap summary is empty, want failure details")
	}

	gapPath := filepath.Join(tmp, ".gromit", "v2", gapFileName)
	data, err := os.ReadFile(gapPath)
	if err != nil {
		t.Fatalf("read gap file: %v", err)
	}
	if string(data) != artifacts.GapSummary {
		t.Fatalf("gap content = %q, want %q", string(data), artifacts.GapSummary)
	}

	if len(artifacts.Results) != 2 {
		t.Fatalf("results count = %d, want 2", len(artifacts.Results))
	}

	if !strings.Contains(llmProvider.calls[0].Prompt, "BASE") || !strings.Contains(llmProvider.calls[0].Prompt, "PROJECT") {
		t.Fatalf("prompt missing configured layers")
	}
	if !strings.Contains(llmProvider.calls[1].Prompt, "diff content") {
		t.Fatalf("prompt missing diff content")
	}
	if git.lastWorktree != tmp {
		t.Fatalf("diff invoked with %q, want %q", git.lastWorktree, tmp)
	}
}

func TestRunProceedWhenAllCriteriaPass(t *testing.T) {
	t.Parallel()

	specID := "spec-pass"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specDir, specID+".md")
	content := "# Pass spec\n\n## Acceptance Criteria\n- done A\n- done B\n"
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths: config.PathsConfig{
			GromitDir: ".gromit",
			Specs:     ".gromit/specs",
		},
	}

	git := &fakeGitAdapter{diff: "pass diff"}
	llmProvider := &fakeLLM{
		responses: []*llm.LLMResponse{
			{Success: true, Output: `{"pass": true, "summary": "done"}`},
			{Success: true, Output: `{"pass": true, "summary": "shipped"}`},
		},
	}

	stageInstance, err := New(cfg, git, llmProvider, "BASE", "PROJECT", "FRAGMENT")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	stageInstance.batchDiffThreshold = -1 // force per-criterion evaluation

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID},
		Worktree: tmp,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionProceed)
	}

	artifacts, ok := res.Artifacts.(*AcceptArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T, want *AcceptArtifacts", res.Artifacts)
	}
	if artifacts.GapSummary != "" {
		t.Fatalf("gap summary = %q, want empty", artifacts.GapSummary)
	}

	gapPath := filepath.Join(tmp, ".gromit", "v2", gapFileName)
	if _, err := os.Stat(gapPath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("gap file exists unexpectedly: %v", err)
	}

	if len(artifacts.Results) != 2 {
		t.Fatalf("results count = %d, want 2", len(artifacts.Results))
	}
	for _, result := range artifacts.Results {
		if !strings.HasPrefix(result.Description, "PASS:") {
			t.Fatalf("result description missing pass marker: %q", result.Description)
		}
	}

	if got := len(llmProvider.calls); got != 2 {
		t.Fatalf("llm calls = %d, want 2", got)
	}
}

type fakeLLM struct {
	calls     []llm.InvokeRequest
	responses []*llm.LLMResponse
}

func (f *fakeLLM) Invoke(ctx context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
	f.calls = append(f.calls, req)
	if len(f.responses) == 0 {
		return nil, fmt.Errorf("no responses")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeLLM) StreamInvoke(ctx context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return nil, fmt.Errorf("stream invoke not supported")
}

type fakeGitAdapter struct {
	diff               string
	diffFromBase       string
	lastWorktree       string
	diffCalled         bool
	diffFromBaseCalled bool
}

func (f *fakeGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) Diff(ctx context.Context, worktree string) (string, error) {
	f.diffCalled = true
	f.lastWorktree = worktree
	return f.diff, nil
}

func (f *fakeGitAdapter) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	f.diffFromBaseCalled = true
	f.lastWorktree = worktree
	if f.diffFromBase != "" {
		return f.diffFromBase, nil
	}
	return f.diff, nil
}

func (f *fakeGitAdapter) Commit(ctx context.Context, worktree, message string) (string, error) {
	f.lastWorktree = worktree
	_ = message
	return "fake", nil
}

func (f *fakeGitAdapter) RemoveWorktree(ctx context.Context, worktree string) error {
	f.lastWorktree = worktree
	return nil
}

func (f *fakeGitAdapter) Status(_ context.Context, worktree string) (string, error) {
	f.lastWorktree = worktree
	return "", nil
}

func setupAcceptStage(t *testing.T, llmProvider *fakeLLM) (*Stage, *stagepkg.Request) {
	t.Helper()
	specID := "spec-err"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specDir, specID+".md")
	content := "# Error spec\n\n## Acceptance Criteria\n- criterion A\n"
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths: config.PathsConfig{
			GromitDir: ".gromit",
			Specs:     ".gromit/specs",
		},
	}

	git := &fakeGitAdapter{diff: "diff content"}
	stageInstance, err := New(cfg, git, llmProvider, "BASE", "PROJECT", "FRAGMENT")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	stageInstance.batchDiffThreshold = -1 // force per-criterion evaluation

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID},
		Worktree: tmp,
	}
	return stageInstance, req
}

func TestRunUsesDiffFromBase(t *testing.T) {
	t.Parallel()

	specID := "spec-diffbase"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specDir, specID+".md")
	content := "# DiffBase spec\n\n## Acceptance Criteria\n- criterion A\n"
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths: config.PathsConfig{
			GromitDir: ".gromit",
			Specs:     ".gromit/specs",
		},
	}

	git := &fakeGitAdapter{diff: "old", diffFromBase: "cumulative diff with feature"}
	llmProvider := &fakeLLM{
		responses: []*llm.LLMResponse{
			{Success: true, Output: `{"pass": true, "summary": "done"}`},
		},
	}

	stageInstance, err := New(cfg, git, llmProvider, "BASE", "PROJECT", "FRAGMENT")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	stageInstance.batchDiffThreshold = -1 // force per-criterion evaluation

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID},
		Worktree: tmp,
	}

	_, err = stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if !git.diffFromBaseCalled {
		t.Fatal("expected DiffFromBase to be called, but it was not")
	}
	if git.diffCalled {
		t.Fatal("expected Diff NOT to be called, but it was")
	}

	// Verify the cumulative diff was used in the prompt.
	if len(llmProvider.calls) != 1 {
		t.Fatalf("llm calls = %d, want 1", len(llmProvider.calls))
	}
	if !strings.Contains(llmProvider.calls[0].Prompt, "cumulative diff with feature") {
		t.Fatal("prompt should contain the DiffFromBase output, not the Diff output")
	}
}

func TestRunReturnsErrorWhenLLMFails(t *testing.T) {
	t.Parallel()
	// Empty responses slice causes fakeLLM.Invoke to return an error.
	llmProvider := &fakeLLM{responses: nil}
	stageInstance, req := setupAcceptStage(t, llmProvider)

	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM returns error, got nil")
	}
	if !strings.Contains(err.Error(), "evaluate criterion") {
		t.Fatalf("error should mention evaluate criterion, got: %v", err)
	}
}

func TestRunReturnsErrorWhenLLMReturnsNil(t *testing.T) {
	t.Parallel()
	// A nil entry in responses causes Invoke to return (nil, nil).
	llmProvider := &fakeLLM{responses: []*llm.LLMResponse{nil}}
	stageInstance, req := setupAcceptStage(t, llmProvider)

	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM returns nil response, got nil")
	}
	if !strings.Contains(err.Error(), "nil response") {
		t.Fatalf("error should mention nil response, got: %v", err)
	}
}

// fakeRetryLLM allows per-call control over responses and errors, and tracks
// how many times Invoke was called.
type fakeRetryLLM struct {
	calls     []llm.InvokeRequest
	responses []retryLLMResult
}

type retryLLMResult struct {
	resp *llm.LLMResponse
	err  error
}

func (f *fakeRetryLLM) Invoke(ctx context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
	idx := len(f.calls)
	f.calls = append(f.calls, req)
	if idx >= len(f.responses) {
		return nil, fmt.Errorf("fakeRetryLLM: no response for call %d", idx)
	}
	r := f.responses[idx]
	return r.resp, r.err
}

func (f *fakeRetryLLM) StreamInvoke(ctx context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return nil, fmt.Errorf("stream invoke not supported")
}

func setupAcceptStageWithProvider(t *testing.T, provider llmtypes.LLMProvider) (*Stage, *stagepkg.Request) {
	t.Helper()
	specID := "spec-retry"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specDir, specID+".md")
	content := "# Retry spec\n\n## Acceptance Criteria\n- criterion A\n"
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths: config.PathsConfig{
			GromitDir: ".gromit",
			Specs:     ".gromit/specs",
		},
	}

	git := &fakeGitAdapter{diff: "diff content"}
	stageInstance, err := New(cfg, git, provider, "BASE", "PROJECT", "FRAGMENT")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	stageInstance.batchDiffThreshold = -1 // force per-criterion evaluation

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID},
		Worktree: tmp,
	}
	return stageInstance, req
}

func TestAcceptStage_RetriesOnParseFailure(t *testing.T) {
	t.Parallel()

	provider := &fakeRetryLLM{
		responses: []retryLLMResult{
			// First call: non-JSON output triggers parse failure.
			{resp: &llm.LLMResponse{Success: true, Output: "this is not json"}},
			// Second call (retry): valid JSON.
			{resp: &llm.LLMResponse{Success: true, Output: `{"pass": true, "summary": "looks good"}`}},
		},
	}

	stageInstance, req := setupAcceptStageWithProvider(t, provider)

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionProceed)
	}

	// Provider should have been called exactly twice: initial + 1 retry.
	if got := len(provider.calls); got != 2 {
		t.Fatalf("provider call count = %d, want 2", got)
	}
}

func TestAcceptStage_RetryUsesRepairPrompt(t *testing.T) {
	t.Parallel()

	badOutput := "The implementation satisfies the criterion because the diff shows good changes."
	provider := &fakeRetryLLM{
		responses: []retryLLMResult{
			// First call: non-JSON output triggers parse failure.
			{resp: &llm.LLMResponse{Success: true, Output: badOutput}},
			// Second call (retry with repair prompt): valid JSON.
			{resp: &llm.LLMResponse{Success: true, Output: `{"pass": true, "summary": "looks good"}`}},
		},
	}

	stageInstance, req := setupAcceptStageWithProvider(t, provider)

	_, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	// The retry prompt should contain the previous bad output, not the original evaluation prompt.
	if got := len(provider.calls); got != 2 {
		t.Fatalf("provider call count = %d, want 2", got)
	}
	retryPrompt := provider.calls[1].Prompt
	if !strings.Contains(retryPrompt, badOutput) {
		t.Fatalf("retry prompt should contain previous output, got: %s", retryPrompt)
	}
	if !strings.Contains(retryPrompt, "not valid JSON") {
		t.Fatalf("retry prompt should mention invalid JSON, got: %s", retryPrompt)
	}
}

func TestAcceptStage_ParseFailureIncludesOutputPreview(t *testing.T) {
	t.Parallel()

	badOutput := "Sure! Here is my evaluation: the criterion is met because the diff shows..."
	provider := &fakeRetryLLM{
		responses: []retryLLMResult{
			// First call: non-JSON.
			{resp: &llm.LLMResponse{Success: true, Output: badOutput}},
			// Second call (retry): still non-JSON, exhausts retries.
			{resp: &llm.LLMResponse{Success: true, Output: badOutput}},
		},
	}

	stageInstance, req := setupAcceptStageWithProvider(t, provider)

	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}

	// The error should contain a truncated preview of the LLM output.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "Sure! Here is my evaluation") {
		t.Fatalf("error should contain output preview, got: %v", errMsg)
	}

	// Provider should have been called exactly twice: initial + 1 retry.
	if got := len(provider.calls); got != 2 {
		t.Fatalf("provider call count = %d, want 2", got)
	}
}

func TestAcceptStage_NoRetryOnProviderError(t *testing.T) {
	t.Parallel()

	provider := &fakeRetryLLM{
		responses: []retryLLMResult{
			// First call: provider returns an error (not a parse failure).
			{resp: nil, err: fmt.Errorf("network timeout")},
			// Second call should never happen.
			{resp: &llm.LLMResponse{Success: true, Output: `{"pass": true, "summary": "ok"}`}},
		},
	}

	stageInstance, req := setupAcceptStageWithProvider(t, provider)

	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from provider failure, got nil")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Fatalf("error should mention provider error, got: %v", err)
	}

	// Provider should have been called only once — no retry on provider error.
	if got := len(provider.calls); got != 1 {
		t.Fatalf("provider call count = %d, want 1 (no retry on provider error)", got)
	}
}
