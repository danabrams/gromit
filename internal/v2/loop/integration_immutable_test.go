//go:build integration

package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	execgit "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	"github.com/danabrams/gromit/internal/v2/presentation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	presentstage "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestIntegrationImmutable_StageSequenceStructuredCommits(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-stage-sequence",
		beads: immutableBeads(
			immutableBead("bead-001", "First bead"),
		),
	})

	assertStructuredStageSequence(t, result, []immutableCommitExpectation{
		{BeadID: "spec", StageName: "present", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "accept", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "review", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "validate", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "build", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "decompose", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "plan", Iteration: 1, Decision: "proceed"},
	})
}

func TestIntegrationImmutable_EventLogCumulativeAcrossCommits(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-events-cumulative",
		beads: immutableBeads(
			immutableBead("bead-001", "First bead"),
		),
	})

	assertEventsCumulativeAcrossCommits(t, result)
}

type immutableSpecConfig struct {
	specID            string
	beads             []*bead.Bead
	enableSquash      bool
	validateDecisions []stagepkg.Decision
	validateRetry     stagepkg.RetryConfig
}

type immutableRunResult struct {
	repoRoot     string
	sourceBranch string
	prBranch     string
}

type immutableCommit struct {
	Hash    string
	Subject string
}

type immutableCommitExpectation struct {
	BeadID    string
	StageName string
	Iteration int
	Decision  string
}

func runImmutableSpec(t *testing.T, cfg immutableSpecConfig) immutableRunResult {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)

	specID := strings.TrimSpace(cfg.specID)
	if specID == "" {
		specID = "immutable-default"
	}

	gitAdapter := newImmutableGitAdapter(repoRoot, filepath.Join(repoRoot, ".gromit", "spec-worktrees"))
	stageCommitter := &pipeline.StageCommitter{Git: gitAdapter}

	typedEmitter := event.NewEmitter()
	t.Cleanup(func() {
		typedEmitter.Close()
	})

	buildStage := &immutableBeadStage{name: "build", delay: 15 * time.Millisecond}
	validateStage := &immutableBeadStage{
		name:       "validate",
		delay:      15 * time.Millisecond,
		decisions:  append([]stagepkg.Decision(nil), cfg.validateDecisions...),
		retry:      cfg.validateRetry,
	}
	reviewStage := &immutableBeadStage{name: "review", delay: 15 * time.Millisecond}
	beadLoop, err := NewBeadLoop(BeadLoopConfig{
		Gate:           newNoopStage("gate"),
		Build:          buildStage,
		Validate:       validateStage,
		Review:         reviewStage,
		Epilogue:       newNoopStage("epilogue"),
		Emitter:        typedEmitter,
		StageCommitter: stageCommitter,
	})
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	summaries := make([]presentation.BeadSummary, 0, len(cfg.beads))
	for _, item := range cfg.beads {
		if item == nil {
			continue
		}
		summaries = append(summaries, presentation.BeadSummary{ID: item.ID, Title: item.Title, Description: item.Description})
	}

	presenter := &immutablePresenter{}
	summaryCtx := &presentstage.SummaryContext{
		Plan:          "immutable plan",
		BeadSummaries: summaries,
	}
	presentOpts := []presentstage.Option{}
	if cfg.enableSquash {
		presentOpts = append(presentOpts, presentstage.WithSquashGit(gitAdapter))
	}
	presentStage, err := presentstage.New(&config.Config{}, presenter, summaryCtx, presentOpts...)
	if err != nil {
		t.Fatalf("present stage: %v", err)
	}

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         gitAdapter,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   presenter,
		},
		&config.Config{},
		noopDependencyGate{},
		WithTypedEmitter(typedEmitter),
		WithStageCommitter(stageCommitter),
		WithPlanStage(&immutablePlanStage{plan: "immutable plan", delay: 15 * time.Millisecond}),
		WithDecomposeStage(&immutableDecomposeStage{beads: immutableBeads(cfg.beads...), delay: 15 * time.Millisecond}),
		WithBeadLoop(beadLoop),
		WithAcceptStage(&immutableAcceptStage{delay: 15 * time.Millisecond}),
		WithPresentStage(presentStage, summaryCtx),
	)
	if err != nil {
		t.Fatalf("NewSpecLoop: %v", err)
	}

	if err := loopInstance.Run(context.Background(), specID, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	return immutableRunResult{
		repoRoot:     repoRoot,
		sourceBranch: presentation.SpecBranchName(specID),
		prBranch:     presenter.specBranch(),
	}
}

func assertStructuredStageSequence(t *testing.T, result immutableRunResult, want []immutableCommitExpectation) {
	t.Helper()

	commits := immutableBranchCommits(t, result.repoRoot, result.sourceBranch, len(want)+4)
	if len(commits) < len(want) {
		t.Fatalf("structured commits = %d, want at least %d", len(commits), len(want))
	}

	for i, expected := range want {
		got := commits[i]
		parsed, ok := pipeline.ParseCommitMessage(got.Subject)
		if !ok {
			t.Fatalf("commit %d subject is not structured: %q", i, got.Subject)
		}
		if parsed.BeadID != expected.BeadID {
			t.Fatalf("commit %d bead_id = %q, want %q", i, parsed.BeadID, expected.BeadID)
		}
		if parsed.StageName != expected.StageName {
			t.Fatalf("commit %d stage = %q, want %q", i, parsed.StageName, expected.StageName)
		}
		if parsed.Iteration != expected.Iteration {
			t.Fatalf("commit %d iteration = %d, want %d", i, parsed.Iteration, expected.Iteration)
		}
		if parsed.Decision != expected.Decision {
			t.Fatalf("commit %d decision = %q, want %q", i, parsed.Decision, expected.Decision)
		}
	}
}

func assertEventsCumulativeAcrossCommits(t *testing.T, result immutableRunResult) {
	t.Helper()

	commits := immutableBranchCommits(t, result.repoRoot, result.sourceBranch, 64)
	structured := make([]immutableCommit, 0, len(commits))
	for _, commit := range commits {
		if _, ok := pipeline.ParseCommitMessage(commit.Subject); ok {
			structured = append(structured, commit)
		}
	}
	if len(structured) == 0 {
		t.Fatal("expected structured commits, got none")
	}

	for i, j := 0, len(structured)-1; i < j; i, j = i+1, j-1 {
		structured[i], structured[j] = structured[j], structured[i]
	}

	var previous []string
	for idx, commit := range structured {
		content := immutableShowFileAtCommit(t, result.repoRoot, commit.Hash, ".gromit/v2/events.jsonl")
		lines := immutableJSONLLines(t, content)
		if len(lines) == 0 {
			t.Fatalf("commit %s has empty events.jsonl snapshot", commit.Hash)
		}
		if len(lines) < len(previous) {
			t.Fatalf("events line count regressed at commit %d (%s): got %d, previous %d", idx, commit.Hash, len(lines), len(previous))
		}
		for i, prev := range previous {
			if lines[i] != prev {
				t.Fatalf("events snapshot diverged at commit %d (%s), line %d", idx, commit.Hash, i)
			}
		}
		previous = lines
	}
}

func immutableBead(id, title string) *bead.Bead {
	return &bead.Bead{ID: id, Title: title}
}

func immutableBeads(items ...*bead.Bead) []*bead.Bead {
	cloned := make([]*bead.Bead, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		cloned = append(cloned, &copyItem)
	}
	return cloned
}

func immutableBranchCommits(t *testing.T, repoRoot, branch string, n int) []immutableCommit {
	t.Helper()

	if n <= 0 {
		n = 20
	}
	out := gitCommand(t, repoRoot, "log", "--format=%H%x00%s", "-n", strconv.Itoa(n), branch)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	commits := make([]immutableCommit, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		commits = append(commits, immutableCommit{Hash: parts[0], Subject: parts[1]})
	}
	return commits
}

func immutableShowFileAtCommit(t *testing.T, repoRoot, hash, filePath string) string {
	t.Helper()
	return gitCommand(t, repoRoot, "show", hash+":"+filePath)
}

func immutableJSONLLines(t *testing.T, content string) []string {
	t.Helper()

	trimmed := strings.TrimRight(content, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Fatalf("events.jsonl line %d is invalid JSON: %v", i, err)
		}
	}
	return lines
}

type immutableGitAdapter struct {
	inner *execgit.ExecGitAdapter
}

func newImmutableGitAdapter(repoRoot, worktreesDir string) *immutableGitAdapter {
	return &immutableGitAdapter{inner: execgit.NewExecGitAdapter(repoRoot, worktreesDir)}
}

func (a *immutableGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	return a.inner.Checkout(ctx, specID)
}

func (a *immutableGitAdapter) Diff(ctx context.Context, worktree string) (string, error) {
	return a.inner.Diff(ctx, worktree)
}

func (a *immutableGitAdapter) Commit(ctx context.Context, worktree, message string) (string, error) {
	return a.inner.Commit(ctx, worktree, message)
}

func (a *immutableGitAdapter) RemoveWorktree(ctx context.Context, worktree string) error {
	return a.inner.RemoveWorktree(ctx, worktree)
}

func (a *immutableGitAdapter) Status(ctx context.Context, worktree string) (string, error) {
	return a.inner.Status(ctx, worktree)
}

func (a *immutableGitAdapter) Log(ctx context.Context, worktree string, n int) ([]adapter.LogEntry, error) {
	return a.inner.Log(ctx, worktree, n)
}

func (a *immutableGitAdapter) Show(ctx context.Context, worktree, hash string) (string, error) {
	return a.inner.Show(ctx, worktree, hash)
}

func (a *immutableGitAdapter) SquashCommits(ctx context.Context, worktree string, count int) error {
	return a.inner.SquashCommits(ctx, worktree, count)
}

type immutablePlanStage struct {
	plan  string
	delay time.Duration
}

func (s *immutablePlanStage) Name() string { return "plan" }

func (s *immutablePlanStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &planstage.PlanArtifacts{Plan: s.plan},
	}, nil
}

type immutableDecomposeStage struct {
	beads []*bead.Bead
	delay time.Duration
}

func (s *immutableDecomposeStage) Name() string { return "decompose" }

func (s *immutableDecomposeStage) Run(_ context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	if err := immutableAppendLine(filepath.Join(req.Worktree, "spec", "decompose.log"), "decompose"); err != nil {
		return nil, err
	}
	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &stagepkg.DecomposeArtifacts{Beads: immutableBeads(s.beads...)},
	}, nil
}

type immutableAcceptStage struct {
	delay time.Duration
}

func (s *immutableAcceptStage) Name() string { return "accept" }

func (s *immutableAcceptStage) Run(_ context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	if err := immutableAppendLine(filepath.Join(req.Worktree, "spec", "accept.log"), "accept"); err != nil {
		return nil, err
	}
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

type immutableBeadStage struct {
	name      string
	delay     time.Duration
	decisions []stagepkg.Decision
	retry     stagepkg.RetryConfig
	mu        sync.Mutex
	calls     int
}

func (s *immutableBeadStage) Name() string { return s.name }

func (s *immutableBeadStage) RetryConfig() stagepkg.RetryConfig {
	return s.retry
}

func (s *immutableBeadStage) Run(_ context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)

	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()

	if err := immutableAppendLine(filepath.Join(req.Worktree, "beads", req.Bead.ID+".log"), fmt.Sprintf("%s-%d", s.name, call)); err != nil {
		return nil, err
	}

	decision := stagepkg.DecisionProceed
	if call <= len(s.decisions) && s.decisions[call-1] != 0 {
		decision = s.decisions[call-1]
	}
	return &stagepkg.Result{Decision: decision}, nil
}

type immutablePresenter struct {
	mu          sync.Mutex
	lastSummary presentation.PresentationSummary
}

func (p *immutablePresenter) PresentSummary(_ context.Context, specID string, summary presentation.PresentationSummary) error {
	if err := immutableAppendLine(filepath.Join(summary.Worktree, "spec", "present.log"), specID); err != nil {
		return err
	}
	p.mu.Lock()
	p.lastSummary = summary
	p.mu.Unlock()
	return nil
}

func (p *immutablePresenter) specBranch() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastSummary.SpecBranch
}

func immutableAppendLine(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}
