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

func TestIntegrationImmutable_CommitPerStageFlowCreatesStructuredCommits(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-stage-sequence",
		beads: immutableBeads(
			immutableBead("bead-001", "First bead"),
		),
	})

	assertStructuredStageSequence(t, result, []immutableCommitExpectation{
		{BeadID: "bead-001", StageName: "review", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "validate", Iteration: 1, Decision: "proceed"},
		{BeadID: "bead-001", StageName: "build", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "decompose", Iteration: 1, Decision: "proceed"},
		{BeadID: "spec", StageName: "plan", Iteration: 1, Decision: "proceed"},
	})
}

func TestIntegrationImmutable_StructuredCommitsFollowStageOrderPerBead(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-stage-order-multi-bead",
		beads: immutableBeads(
			immutableBead("bead-001", "First bead"),
			immutableBead("bead-002", "Second bead"),
		),
	})

	commits := collectStructuredStageCommits(t, result.repoRoot, result.sourceBranch, 64)
	assertStageSequencePerBead(t, commits, []string{"build", "validate", "review"}, []string{"bead-001", "bead-002"})
}

func TestIntegrationImmutable_EventLogSnapshotsRemainCumulative(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-events-cumulative",
		beads: immutableBeads(
			immutableBead("bead-001", "First bead"),
		),
	})

	assertEventsCumulativeAcrossCommits(t, result)
}

func TestIntegrationImmutable_RetryHistoryKeepsPreviousCommits(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID: "immutable-retry-history",
		beads: immutableBeads(
			immutableBead("bead-001", "Retry bead"),
		),
		validateDecisions: []stagepkg.Decision{stagepkg.DecisionFail, stagepkg.DecisionProceed},
		validateRetry: stagepkg.RetryConfig{
			MaxRetries: 1,
			RetryWith:  []string{"build"},
		},
	})

	assertRetryHistoryPreserved(t, result, "bead-001")
}

func TestIntegrationImmutable_PerBeadSquashCombinesStageCommits(t *testing.T) {
	t.Parallel()

	result := runImmutableSpec(t, immutableSpecConfig{
		specID:       "immutable-per-bead-squash",
		enableSquash: true,
		beads: immutableBeads(
			immutableBead("001", "First bead"),
			immutableBead("002", "Second bead"),
		),
	})

	assertPerBeadSquashHistory(t, result, []string{
		"bead 002: Second bead",
		"bead 001: First bead",
	})
}

type immutableSpecConfig struct {
	specID            string
	beads             []*bead.Bead
	enableSquash      bool
	validateDecisions []stagepkg.Decision
	validateRetry     stagepkg.RetryConfig
}

type immutableRunResult struct {
	specID       string
	repoRoot     string
	sourceBranch string
	prBranch     string
}

type immutableCommit struct {
	Hash    string
	Subject string
}

type immutableStructuredCommit struct {
	Commit immutableCommit
	Info   pipeline.CommitInfo
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
		name:      "validate",
		delay:     15 * time.Millisecond,
		decisions: append([]stagepkg.Decision(nil), cfg.validateDecisions...),
		retry:     cfg.validateRetry,
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
		specID:       specID,
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

func assertRetryHistoryPreserved(t *testing.T, result immutableRunResult, beadID string) {
	t.Helper()

	commits := immutableBranchCommits(t, result.repoRoot, result.sourceBranch, 64)
	if len(commits) == 0 {
		t.Fatal("expected source branch commits, got none")
	}
	buildIter1 := -1
	buildIter1Hash := ""
	buildIter2 := -1
	buildIter2Hash := ""
	validateIter1Fail := -1
	validateIter2Proceed := -1

	for idx, commit := range commits {
		parsed, ok := pipeline.ParseCommitMessage(commit.Subject)
		if !ok || parsed.BeadID != beadID {
			continue
		}
		if parsed.StageName == "build" && parsed.Iteration == 1 && parsed.Decision == "proceed" {
			buildIter1 = idx
			buildIter1Hash = commit.Hash
		}
		if parsed.StageName == "build" && parsed.Iteration == 2 && parsed.Decision == "proceed" {
			buildIter2 = idx
			buildIter2Hash = commit.Hash
		}
		if parsed.StageName == "validate" && parsed.Iteration == 1 && parsed.Decision == "fail" {
			validateIter1Fail = idx
		}
		if parsed.StageName == "validate" && parsed.Iteration == 2 && parsed.Decision == "proceed" {
			validateIter2Proceed = idx
		}
	}

	if buildIter1 < 0 {
		t.Fatalf("missing build iteration 1 commit for bead %s", beadID)
	}
	if buildIter2 < 0 {
		t.Fatalf("missing build iteration 2 commit for bead %s", beadID)
	}
	if validateIter1Fail < 0 {
		t.Fatalf("missing validate iteration 1 fail commit for bead %s", beadID)
	}
	if validateIter2Proceed < 0 {
		t.Fatalf("missing validate iteration 2 proceed commit for bead %s", beadID)
	}
	if buildIter2 >= buildIter1 {
		t.Fatalf("build iteration ordering invalid: iter2 index=%d, iter1 index=%d", buildIter2, buildIter1)
	}

	buildIter1Snapshot := immutableShowFileAtCommit(t, result.repoRoot, buildIter1Hash, filepath.Join("beads", beadID+".log"))
	if !strings.Contains(buildIter1Snapshot, "build-1") {
		t.Fatalf("build iteration 1 snapshot missing build-1 output: %q", buildIter1Snapshot)
	}
	if strings.Contains(buildIter1Snapshot, "build-2") {
		t.Fatalf("build iteration 1 snapshot unexpectedly contains build-2 output: %q", buildIter1Snapshot)
	}

	buildIter2Snapshot := immutableShowFileAtCommit(t, result.repoRoot, buildIter2Hash, filepath.Join("beads", beadID+".log"))
	if !strings.Contains(buildIter2Snapshot, "build-1") || !strings.Contains(buildIter2Snapshot, "build-2") {
		t.Fatalf("build iteration 2 snapshot missing retry history output: %q", buildIter2Snapshot)
	}

	eventsContent := immutableShowFileAtCommit(t, result.repoRoot, commits[0].Hash, ".gromit/v2/events.jsonl")
	lines := immutableJSONLLines(t, eventsContent)
	retryEventFound := false
	for i, line := range lines {
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("events line %d decode: %v", i, err)
		}
		if decoded["type"] != event.EventTypeStageRetrying {
			continue
		}
		if decoded["bead_id"] != beadID || decoded["stage_name"] != "validate" {
			continue
		}
		retryEventFound = true
		attempt, ok := decoded["attempt"].(float64)
		if !ok {
			t.Fatalf("retry event attempt type = %T, want number", decoded["attempt"])
		}
		if int(attempt) != 2 {
			t.Fatalf("retry event attempt = %v, want 2", decoded["attempt"])
		}
		iteration, ok := decoded["iteration"].(float64)
		if !ok {
			t.Fatalf("retry event iteration type = %T, want number", decoded["iteration"])
		}
		if int(iteration) != 2 {
			t.Fatalf("retry event iteration = %v, want 2", decoded["iteration"])
		}
	}
	if !retryEventFound {
		t.Fatalf("missing stage.retrying event for bead %s", beadID)
	}
}

func assertPerBeadSquashHistory(t *testing.T, result immutableRunResult, wantBeadSubjects []string) {
	t.Helper()

	wantPRBranch := presentation.SpecPRBranchName(result.specID)
	if result.prBranch != wantPRBranch {
		t.Fatalf("presented spec branch = %q, want %q", result.prBranch, wantPRBranch)
	}

	sourceCommits := immutableBranchCommits(t, result.repoRoot, result.sourceBranch, 64)
	foundStructured := false
	for _, commit := range sourceCommits {
		if _, ok := pipeline.ParseCommitMessage(commit.Subject); ok {
			foundStructured = true
			break
		}
	}
	if !foundStructured {
		t.Fatalf("source branch %q lost structured stage commits", result.sourceBranch)
	}

	prCommits := immutableBranchCommits(t, result.repoRoot, result.prBranch, len(wantBeadSubjects)+4)
	if len(prCommits) < len(wantBeadSubjects) {
		t.Fatalf("pr branch commits = %d, want at least %d", len(prCommits), len(wantBeadSubjects))
	}

	for i, commit := range prCommits {
		if _, ok := pipeline.ParseCommitMessage(commit.Subject); ok {
			t.Fatalf("pr commit %d should be non-structured after squash, got %q", i, commit.Subject)
		}
	}

	lastIndex := -1
	for _, want := range wantBeadSubjects {
		found := -1
		for idx, commit := range prCommits {
			if commit.Subject == want {
				found = idx
				break
			}
		}
		if found < 0 {
			t.Fatalf("pr branch missing squashed bead commit %q, commits=%v", want, subjects(prCommits))
		}
		if found <= lastIndex {
			t.Fatalf("squashed bead commit order incorrect for %q: index=%d previous=%d", want, found, lastIndex)
		}
		lastIndex = found
	}
}

func subjects(commits []immutableCommit) []string {
	out := make([]string, 0, len(commits))
	for _, commit := range commits {
		out = append(out, commit.Subject)
	}
	return out
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

func collectStructuredStageCommits(t *testing.T, repoRoot, branch string, n int) []immutableStructuredCommit {
	t.Helper()

	if n <= 0 {
		n = 64
	}
	commits := immutableBranchCommits(t, repoRoot, branch, n)
	structured := make([]immutableStructuredCommit, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		if info, ok := pipeline.ParseCommitMessage(commits[i].Subject); ok {
			structured = append(structured, immutableStructuredCommit{Commit: commits[i], Info: info})
		}
	}
	return structured
}

func assertStageSequencePerBead(t *testing.T, commits []immutableStructuredCommit, stageOrder, beadIDs []string) {
	t.Helper()

	if len(stageOrder) == 0 {
		t.Fatalf("no stage order provided")
	}
	expected := make(map[string]int, len(beadIDs))
	for _, beadID := range beadIDs {
		expected[beadID] = 0
	}

	for _, entry := range commits {
		beadID := entry.Info.BeadID
		if beadID == "spec" {
			continue
		}
		nextIdx, ok := expected[beadID]
		if !ok {
			t.Fatalf("unexpected bead ID %q in structured commits", beadID)
		}
		if nextIdx >= len(stageOrder) {
			t.Fatalf("extra stage commit %q for bead %q beyond expected stages", entry.Info.StageName, beadID)
		}
		if entry.Info.StageName != stageOrder[nextIdx] {
			t.Fatalf("bead %q stage order mismatch: got %q (index %d), want %q", beadID, entry.Info.StageName, nextIdx, stageOrder[nextIdx])
		}
		expected[beadID] = nextIdx + 1
	}

	for _, beadID := range beadIDs {
		if expected[beadID] != len(stageOrder) {
			t.Fatalf("bead %q completed %d/%d stages", beadID, expected[beadID], len(stageOrder))
		}
	}
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
