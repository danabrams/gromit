//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/testutil"
)

type acceptanceFixture struct {
	Git         *testutil.FakeGit
	TaskTracker *testutil.FakeTaskTracker
	Presenter   *testutil.FakePresenter
	llmAdapter  *acceptanceLLMAdapter
}

func newAcceptanceAdapters(t *testing.T) *acceptanceFixture {
	t.Helper()
	git := testutil.NewFakeGit()
	git.WorktreeRoot = t.TempDir()
	return &acceptanceFixture{
		Git:         git,
		TaskTracker: testutil.NewFakeTaskTracker(),
		Presenter:   testutil.NewFakePresenter(),
		llmAdapter: &acceptanceLLMAdapter{
			fake: createAcceptanceFakeLLM(),
		},
	}
}

func createAcceptanceFakeLLM() *testutil.FakeLLM {
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{Output: "plan", Success: true})
	return fake
}

func (f *acceptanceFixture) AdapterSet() adapter.AdapterSet {
	return adapter.AdapterSet{
		Git:         f.Git,
		LLM:         f.llmAdapter,
		TaskTracker: f.TaskTracker,
		Presenter:   f.Presenter,
	}
}

func (f *acceptanceFixture) Worktree(specID string) string {
	return filepath.Join(f.Git.WorktreeRoot, specID)
}

func (f *acceptanceFixture) LLMFake() *testutil.FakeLLM {
	return f.llmAdapter.fake
}

type acceptanceLLMAdapter struct {
	fake *testutil.FakeLLM
}

func (a *acceptanceLLMAdapter) GeneratePlan(ctx context.Context, specID string) (string, error) {
	resp, err := a.fake.Invoke(ctx, llm.InvokeRequest{Prompt: specID})
	if err != nil {
		return "", fmt.Errorf("invoke fake llm: %w", err)
	}
	if resp == nil || !resp.Success {
		return "", fmt.Errorf("fake llm response unsuccessful")
	}
	return fmt.Sprintf("%s-plan", specID), nil
}
