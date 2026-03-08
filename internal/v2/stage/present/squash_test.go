package present

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

type fakePresentSquashGit struct {
	logEntries  []adapter.LogEntry
	logErr      error
	squashCalls []int
	squashErr   error
	commitCalls []string
	commitErr   error
}

func (f *fakePresentSquashGit) Log(_ context.Context, _ string, _ int) ([]adapter.LogEntry, error) {
	if f.logErr != nil {
		return nil, f.logErr
	}
	return f.logEntries, nil
}

func (f *fakePresentSquashGit) SquashCommits(_ context.Context, _ string, count int) error {
	f.squashCalls = append(f.squashCalls, count)
	return f.squashErr
}

func (f *fakePresentSquashGit) Commit(_ context.Context, _ string, message string) (string, error) {
	f.commitCalls = append(f.commitCalls, message)
	if f.commitErr != nil {
		return "", f.commitErr
	}
	return "squash-commit", nil
}

func TestSquashPerBead_squashesEachBeadBoundary(t *testing.T) {
	fake := &fakePresentSquashGit{
		logEntries: []adapter.LogEntry{
			{Hash: "h4", Message: "[bead:002/review/iter:1] Proceed"},
			{Hash: "h3", Message: "[bead:002/validate/iter:1] Proceed"},
			{Hash: "h2", Message: "[bead:001/review/iter:1] Proceed"},
			{Hash: "h1", Message: "[bead:001/build/iter:1] Proceed"},
			{Hash: "h0", Message: "initial commit"},
		},
	}
	beads := []presentation.BeadSummary{
		{ID: "001", Title: "First Bead"},
		{ID: "002", Title: "Second Bead"},
	}

	err := squashPerBead(context.Background(), fake, "/tmp/wt", beads)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.squashCalls) != 2 || fake.squashCalls[0] != 2 || fake.squashCalls[1] != 2 {
		t.Errorf("squashCalls = %v, want [2 2]", fake.squashCalls)
	}
	wantMessages := []string{"bead 002: Second Bead", "bead 001: First Bead"}
	if len(fake.commitCalls) != len(wantMessages) {
		t.Fatalf("commitCalls = %v, want %v", fake.commitCalls, wantMessages)
	}
	for i := range wantMessages {
		if fake.commitCalls[i] != wantMessages[i] {
			t.Fatalf("commitCalls[%d] = %q, want %q", i, fake.commitCalls[i], wantMessages[i])
		}
	}
}

func TestBuildSquashSegments_onlyIncludesPerBeadStages(t *testing.T) {
	allowedBeads := map[string]struct{}{
		"001": {},
	}

	prefix := []adapter.LogEntry{
		{Hash: "h6", Message: "[bead:001/present/iter:1] Proceed"},
		{Hash: "h5", Message: "[bead:001/decompose/iter:1] Proceed"},
		{Hash: "h4", Message: "[bead:001/review/iter:1] Proceed"},
		{Hash: "h3", Message: "[bead:001/validate/iter:1] Proceed"},
		{Hash: "h2", Message: "[bead:001/build/iter:1] Proceed"},
		{Hash: "h1", Message: "[bead:001/gate/iter:1] Proceed"},
	}

	segments := buildSquashSegments(prefix, allowedBeads)
	if len(segments) != 1 {
		t.Fatalf("segments len = %d, want 1", len(segments))
	}
	if segments[0].beadID != "001" {
		t.Fatalf("segment beadID = %q, want %q", segments[0].beadID, "001")
	}

	wantHashes := []string{"h2", "h3", "h4", "h6"}
	if len(segments[0].hashes) != len(wantHashes) {
		t.Fatalf("segment hashes len = %d, want %d (%v)", len(segments[0].hashes), len(wantHashes), segments[0].hashes)
	}
	for i := range wantHashes {
		if segments[0].hashes[i] != wantHashes[i] {
			t.Fatalf("segment hashes[%d] = %q, want %q", i, segments[0].hashes[i], wantHashes[i])
		}
	}
}

func TestBuildSquashSegments_groupsInterleavedCommitsByBeadID(t *testing.T) {
	allowedBeads := map[string]struct{}{
		"001": {},
		"002": {},
	}

	prefix := []adapter.LogEntry{
		{Hash: "h6", Message: "[bead:001/review/iter:1] Proceed"},
		{Hash: "h5", Message: "[bead:002/validate/iter:1] Proceed"},
		{Hash: "h4", Message: "[bead:001/validate/iter:1] Proceed"},
		{Hash: "h3", Message: "[bead:002/build/iter:1] Proceed"},
		{Hash: "h2", Message: "[bead:001/build/iter:1] Proceed"},
		{Hash: "h1", Message: "[bead:spec/plan/iter:1] Proceed"},
	}

	segments := buildSquashSegments(prefix, allowedBeads)
	if len(segments) != 2 {
		t.Fatalf("segments len = %d, want 2 (%v)", len(segments), segments)
	}

	if segments[0].beadID != "001" {
		t.Fatalf("segments[0].beadID = %q, want %q", segments[0].beadID, "001")
	}
	want001 := []string{"h2", "h4", "h6"}
	if len(segments[0].hashes) != len(want001) {
		t.Fatalf("segments[0].hashes len = %d, want %d", len(segments[0].hashes), len(want001))
	}
	for i := range want001 {
		if segments[0].hashes[i] != want001[i] {
			t.Fatalf("segments[0].hashes[%d] = %q, want %q", i, segments[0].hashes[i], want001[i])
		}
	}

	if segments[1].beadID != "002" {
		t.Fatalf("segments[1].beadID = %q, want %q", segments[1].beadID, "002")
	}
	want002 := []string{"h3", "h5"}
	if len(segments[1].hashes) != len(want002) {
		t.Fatalf("segments[1].hashes len = %d, want %d", len(segments[1].hashes), len(want002))
	}
	for i := range want002 {
		if segments[1].hashes[i] != want002[i] {
			t.Fatalf("segments[1].hashes[%d] = %q, want %q", i, segments[1].hashes[i], want002[i])
		}
	}
}

func TestCollectBeadCommitHashes_combinesInterleavedStageCommits(t *testing.T) {
	entries := []adapter.LogEntry{
		{Hash: "h6", Message: "[bead:001/review/iter:1] Proceed"},
		{Hash: "h5", Message: "[bead:002/validate/iter:1] Proceed"},
		{Hash: "h4", Message: "[bead:001/validate/iter:1] Proceed"},
		{Hash: "h3", Message: "[bead:002/build/iter:1] Proceed"},
		{Hash: "h2", Message: "[bead:001/build/iter:1] Proceed"},
		{Hash: "h1", Message: "[bead:spec/plan/iter:1] Proceed"},
	}
	allowedBeads := map[string]struct{}{
		"001": {},
		"002": {},
	}

	order, hashes := collectBeadCommitHashes(entries, allowedBeads)
	wantOrder := []string{"001", "002"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("order = %v, want %v", order, wantOrder)
	}

	wantHashes := map[string][]string{
		"001": {"h2", "h4", "h6"},
		"002": {"h3", "h5"},
	}
	if !reflect.DeepEqual(hashes, wantHashes) {
		t.Fatalf("hashes = %v, want %v", hashes, wantHashes)
	}
}

func TestCollectBeadCommitHashes_respectsAllowedBeads(t *testing.T) {
	entries := []adapter.LogEntry{
		{Hash: "h3", Message: "[bead:002/build/iter:1] Proceed"},
		{Hash: "h2", Message: "[bead:001/validate/iter:1] Proceed"},
		{Hash: "h1", Message: "[bead:001/build/iter:1] Proceed"},
	}
	allowedBeads := map[string]struct{}{
		"001": {},
	}

	order, hashes := collectBeadCommitHashes(entries, allowedBeads)
	wantOrder := []string{"001"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("order = %v, want %v", order, wantOrder)
	}
	if len(hashes) != 1 || !reflect.DeepEqual(hashes["001"], []string{"h1", "h2"}) {
		t.Fatalf("hashes = %v, want only bead 001 %v", hashes, []string{"h1", "h2"})
	}
}

func TestSquashPerBeadForPresentation_preservesWorktreeHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := initPresentTestRepo(t)
	worktreesDir := t.TempDir()
	gitAdapter := git.NewExecGitAdapter(repoDir, worktreesDir)
	committer := &pipeline.StageCommitter{Git: gitAdapter}

	const specID = "spec-squash-history"
	wtPath, err := gitAdapter.Checkout(context.Background(), specID)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	writeFile(t, filepath.Join(wtPath, "bead.txt"), "build stage")
	if err := committer.CommitStage(context.Background(), wtPath, "001", "build", 1, "Proceed"); err != nil {
		t.Fatalf("commit build stage: %v", err)
	}
	writeFile(t, filepath.Join(wtPath, "bead.txt"), "review stage")
	if err := committer.CommitStage(context.Background(), wtPath, "001", "review", 1, "Proceed"); err != nil {
		t.Fatalf("commit review stage: %v", err)
	}

	specBranch := presentation.SpecBranchName(specID)
	beforeSubjects := logSubjects(t, wtPath, specBranch, 4)

	branch, err := squashPerBeadForPresentation(context.Background(), gitAdapter, wtPath, specID, []presentation.BeadSummary{{ID: "001", Title: "Feature"}})
	if err != nil {
		t.Fatalf("squash per bead: %v", err)
	}
	if branch == "" {
		t.Fatal("expected PR branch")
	}

	if got, want := strings.TrimSpace(runGitInDir(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD")), specBranch; got != want {
		t.Fatalf("worktree branch = %q, want %q", got, want)
	}

	afterSubjects := logSubjects(t, wtPath, specBranch, len(beforeSubjects))
	if !reflect.DeepEqual(beforeSubjects, afterSubjects) {
		t.Fatalf("worktree history changed: before=%v after=%v", beforeSubjects, afterSubjects)
	}

	if got, want := branch, presentation.SpecPRBranchName(specID); got != want {
		t.Fatalf("pr branch = %q, want %q", got, want)
	}

	prSubjects := logSubjects(t, wtPath, branch, 2)
	if len(prSubjects) == 0 || prSubjects[0] != "bead 001: Feature" {
		t.Fatalf("pr branch head commit = %v, want bead squash message", prSubjects)
	}
}

func logSubjects(t *testing.T, worktree, branch string, limit int) []string {
	t.Helper()
	if limit <= 0 {
		limit = 1
	}
	args := []string{"log", "--format=%s", branch, fmt.Sprintf("-%d", limit)}
	out := strings.TrimSpace(runGitInDir(t, worktree, args...))
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	subjects := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		subjects = append(subjects, trimmed)
	}
	return subjects
}
