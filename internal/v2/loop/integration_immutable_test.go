//go:build integration
// +build integration

package loop

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/events"
    "github.com/danabrams/gromit/internal/v2/adapter"
    gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
    "github.com/danabrams/gromit/internal/v2/event"
    "github.com/danabrams/gromit/internal/v2/pipeline"
    "github.com/danabrams/gromit/internal/v2/presentation"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestIntegrationSpecLoopStageCommitMessages(t *testing.T) {
    requireGit(t)
    t.Parallel()

    specID := "integration-stage-commit"
    repoRoot := initIntegrationRepo(t)
    worktreesDir := t.TempDir()
    gp := gitadapter.NewExecGitAdapter(repoRoot, worktreesDir)
    sc := &pipeline.StageCommitter{Git: gp}

    typedEmitter := event.NewEmitter()
    t.Cleanup(typedEmitter.Close)

    planStage := newFakePlanStage(specID)
    decomposeStage := newFakeDecomposeStage(specID)
    worktreePath := filepath.Join(worktreesDir, specID)
    decomposeStage.onRun = func() {
        writeTestFile(t, worktreePath, "decompose.txt", "decompose output")
    }

    beadLoop, err := NewBeadLoop(BeadLoopConfig{
        Gate:          newFileWritingStage("gate"),
        Build:         newFileWritingStage("build"),
        Validate:      newFileWritingStage("validate"),
        Review:        newFileWritingStage("review"),
        Epilogue:      newNoopStage("epilogue"),
        Git:           gp,
        StageCommitter: sc,
        Emitter:       typedEmitter,
    })
    if err != nil {
        t.Fatalf("create bead loop: %v", err)
    }

    presenter := &fileWritingPresenter{}
    summaryCtx := &present.SummaryContext{}
    presentStage, err := present.New(&config.Config{}, presenter, summaryCtx)
    if err != nil {
        t.Fatalf("create present stage: %v", err)
    }

    adapters := adapter.AdapterSet{
        Git:         gp,
        LLM:         newIntegrationLLMAdapter(),
        TaskTracker: newIntegrationTaskTrackerAdapter(),
        Presenter:   presenter,
    }

    emitter := events.NewEmitter()

    loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
        WithEmitter(emitter),
        WithTypedEmitter(typedEmitter),
        WithPlanStage(planStage),
        WithDecomposeStage(decomposeStage),
        WithBeadLoop(beadLoop),
        WithAcceptStage(newFileWritingStage("accept")),
        WithPresentStage(presentStage, summaryCtx),
        WithStageCommitter(sc),
    )
    if err != nil {
        t.Fatalf("create spec loop: %v", err)
    }

    if err := loopInstance.Run(context.Background(), specID, nil); err != nil {
        t.Fatalf("run spec loop: %v", err)
    }

    branch := fmt.Sprintf("gromit/spec/%s", specID)
    expectedStages := []string{"present", "accept", "review", "validate", "build", "gate", "decompose", "plan"}
    entries := gitLogEntries(t, repoRoot, branch, len(expectedStages))
    if len(entries) < len(expectedStages) {
        t.Fatalf("too few commits: got %d, want at least %d", len(entries), len(expectedStages))
    }

    for i, stageName := range expectedStages {
        info, ok := pipeline.ParseCommitMessage(entries[i].Message)
        if !ok {
            t.Fatalf("commit %d message not parseable: %q", i, entries[i].Message)
        }
        if info.StageName != stageName {
            t.Fatalf("stage %d = %q, want %q", i, info.StageName, stageName)
        }
        if info.Decision != "Proceed" {
            t.Fatalf("stage %s decision = %q, want Proceed", stageName, info.Decision)
        }
    }
}

func writeTestFile(t *testing.T, worktree, name, content string) {
    t.Helper()
    if err := writeFile(worktree, name, content); err != nil {
        t.Fatalf("write %s: %v", name, err)
    }
}

func newFileWritingStage(name string) stagepkg.Stage {
    return &fileWritingStage{name: name}
}

type fileWritingStage struct {
    name string
}

func (s *fileWritingStage) Name() string { return s.name }

func (s *fileWritingStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
    if req == nil {
        return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
    }
    if err := writeFile(req.Worktree, fmt.Sprintf("%s-%s.txt", s.name, req.Bead.ID), fmt.Sprintf("%s run", s.name)); err != nil {
        return nil, err
    }
    return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func writeFile(worktree, name, content string) error {
    if strings.TrimSpace(worktree) == "" {
        return fmt.Errorf("worktree required")
    }
    path := filepath.Join(worktree, ".gromit", "v2", name)
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return os.WriteFile(path, []byte(content), 0o644)
}

type fileWritingPresenter struct{}

func (f *fileWritingPresenter) PresentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
    if summary.Worktree == "" {
        return fmt.Errorf("worktree required")
    }
    return writeFile(summary.Worktree, "present.txt", fmt.Sprintf("presented %s", specID))
}

func initIntegrationRepo(t *testing.T) string {
    t.Helper()
    if _, err := exec.LookPath("git"); err != nil {
        t.Skip("git not available")
    }
    repoDir := t.TempDir()
    runGit(t, repoDir, "init")
    runGit(t, repoDir, "config", "user.email", "test@example.com")
    runGit(t, repoDir, "config", "user.name", "Test User")
    runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
    return repoDir
}

func runGit(t *testing.T, dir string, args ...string) {
    t.Helper()
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("git %v: %v\n%s", args, err, out)
    }
}

func gitLogEntries(t *testing.T, repoRoot, ref string, n int) []adapter.LogEntry {
    t.Helper()
    args := []string{"log", "--format=%H%x00%s", "-" + strconv.Itoa(n), ref}
    cmd := exec.Command("git", args...)
    cmd.Dir = repoRoot
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("git log %s: %v\n%s", ref, err, out)
    }
    var entries []adapter.LogEntry
    for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
        if line == "" {
            continue
        }
        parts := strings.SplitN(line, "\x00", 2)
        if len(parts) != 2 {
            continue
        }
        entries = append(entries, adapter.LogEntry{Hash: parts[0], Message: parts[1]})
    }
    return entries
}

func requireGit(t *testing.T) {
    t.Helper()
    if _, err := exec.LookPath("git"); err != nil {
        t.Skip("git not available")
    }
}
