package next_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/artifact"
	"github.com/danabrams/gromit/internal/next/contextpkt"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/extract"
	"github.com/danabrams/gromit/internal/next/guide"
	"github.com/danabrams/gromit/internal/next/infer"
	"github.com/danabrams/gromit/internal/next/inspect"
	"github.com/danabrams/gromit/internal/next/projectcell"
	"github.com/danabrams/gromit/internal/next/provenance"
	"github.com/danabrams/gromit/internal/next/sourcemap"
	"github.com/danabrams/gromit/internal/next/workspace"
)

func TestIntegration_FullProjectCellFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Create temp workspace via GROMIT_HOME
	workspaceDir := t.TempDir()
	t.Setenv("GROMIT_HOME", workspaceDir)

	// 2. Create a fixture git repo with known files
	fixtureRepo := createFixtureRepo(t)

	// 3. Resolve workspace
	resolver := workspace.NewEnvResolver()
	root, err := resolver.Resolve()
	if err != nil {
		t.Fatalf("resolve workspace: %v", err)
	}
	if string(root) != workspaceDir {
		t.Fatalf("root = %q, want %q", root, workspaceDir)
	}

	// 4. Attach the fixture repo as a project
	store := projectcell.NewFSStore(root.ProjectsDir())
	cell, err := store.Create("fixture-app", fixtureRepo)
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Verify cell structure
	for _, sub := range []string{"artifacts", "doctrine", "provenance", "guide"} {
		dir := filepath.Join(cell.CellPath, sub)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s", dir)
		}
	}

	// 5. Run inspect with stub inferrer
	extractors := []inspect.Extractor{
		extract.NewFileTreeExtractor(),
		extract.NewGoModExtractor(),
		extract.NewValidationCommandsExtractor(),
	}
	inferrer := infer.NewStubInferrer()
	inspector := inspect.NewInspector(extractors, inferrer)

	result, err := inspector.Inspect(context.Background(), cell)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(result.Observed) == 0 {
		t.Error("expected observed facts from inspection")
	}

	// Write artifacts
	artStore := artifact.NewJSONStore()
	artifactsDir := filepath.Join(cell.CellPath, "artifacts")

	sm := sourcemap.BuildFromFacts(result.Observed)
	if err := artStore.Write(artifactsDir, "sourcemap", sm); err != nil {
		t.Fatalf("write sourcemap: %v", err)
	}

	// Write architecture artifact (from inferred or empty)
	arch := architecture.New()
	arch.AddModule(architecture.Module{Name: "main", Description: "Entry point", Language: "go"})
	if err := artStore.Write(artifactsDir, "architecture", arch); err != nil {
		t.Fatalf("write architecture: %v", err)
	}

	// Write doctrine
	testDoc := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			doctrine.NewRule("test-001", "TDD required", "testing"),
		},
	}
	docStore := doctrine.NewFSStore()
	if err := docStore.Save(filepath.Join(cell.CellPath, "doctrine"), testDoc); err != nil {
		t.Fatalf("save doctrine: %v", err)
	}
	// Also write doctrine as artifact for context compiler
	if err := artStore.Write(artifactsDir, "doctrine", testDoc); err != nil {
		t.Fatalf("write doctrine artifact: %v", err)
	}

	// Record provenance
	tracker := provenance.NewFSTracker(filepath.Join(cell.CellPath, "provenance", "provenance.json"))
	tracker.Record(provenance.Record{
		FactID:   "sourcemap",
		Artifact: "sourcemap",
		Category: "observed",
		GitSHA:   "abc123",
	})

	// Verify artifacts exist
	if !artStore.Exists(artifactsDir, "sourcemap") {
		t.Error("sourcemap artifact should exist after inspection")
	}

	// Verify provenance
	rec, err := tracker.Check("sourcemap")
	if err != nil {
		t.Fatalf("Check provenance: %v", err)
	}
	if rec.GitSHA != "abc123" {
		t.Errorf("provenance SHA = %q, want %q", rec.GitSHA, "abc123")
	}

	// 6. Render agent guide
	doc, _ := docStore.Load(filepath.Join(cell.CellPath, "doctrine"))

	renderer := guide.NewMarkdownRenderer()
	guideInput := guide.RenderInput{
		ProjectName: cell.Name,
		SourceMap:   sm,
		Doctrine:    doc,
	}
	output, err := renderer.Render(guideInput)
	if err != nil {
		t.Fatalf("Render guide: %v", err)
	}

	guidePath := filepath.Join(cell.CellPath, "guide", "agent-guide.md")
	if err := os.WriteFile(guidePath, output, 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	// Verify guide contains expected headings
	guideContent := string(output)
	if !strings.Contains(guideContent, "# fixture-app") {
		t.Error("guide should contain project heading")
	}
	if !strings.Contains(guideContent, "## Source Map") {
		t.Error("guide should contain Source Map section")
	}

	// 7. Compile context at project level
	artStoreWrapper := &testArtifactStore{store: artStore, artifactsDir: artifactsDir}
	compiler := contextpkt.NewCompiler(artStoreWrapper)

	projectPacket, err := compiler.Compile(context.Background(), cell, contextpkt.LevelProject, contextpkt.CompileOpts{})
	if err != nil {
		t.Fatalf("Compile project: %v", err)
	}
	if len(projectPacket.Sections) == 0 {
		t.Error("project packet should have sections")
	}

	// Compile at spec level
	specPacket, err := compiler.Compile(context.Background(), cell, contextpkt.LevelSpec, contextpkt.CompileOpts{
		SpecPath: "specs/001-feature.md",
	})
	if err != nil {
		t.Fatalf("Compile spec: %v", err)
	}
	hasSpecText := false
	for _, s := range specPacket.Sections {
		if s.Name == "spec-text" {
			hasSpecText = true
		}
	}
	if !hasSpecText {
		t.Error("spec packet should include spec-text section")
	}

	// Compile at task level
	taskPacket, err := compiler.Compile(context.Background(), cell, contextpkt.LevelTask, contextpkt.CompileOpts{
		SpecPath: "specs/001-feature.md",
		TaskID:   "task-1",
	})
	if err != nil {
		t.Fatalf("Compile task: %v", err)
	}
	hasProof := false
	for _, s := range taskPacket.Sections {
		if s.Name == "proof-requirements" {
			hasProof = true
		}
	}
	if !hasProof {
		t.Error("task packet should include proof-requirements section")
	}

	// 8. Verify re-inspect freshness
	fresh, err := tracker.IsFresh("sourcemap", "abc123")
	if err != nil {
		t.Fatalf("IsFresh: %v", err)
	}
	if !fresh {
		t.Error("sourcemap should be fresh with same SHA")
	}

	stale, _ := tracker.IsFresh("sourcemap", "def456")
	if stale {
		t.Error("sourcemap should not be fresh with different SHA")
	}

	// 9. Attach a second project, verify isolation
	fixtureRepo2 := createFixtureRepo(t)
	cell2, err := store.Create("other-app", fixtureRepo2)
	if err != nil {
		t.Fatalf("Create second project: %v", err)
	}

	// Inspect second project
	result2, err := inspector.Inspect(context.Background(), cell2)
	if err != nil {
		t.Fatalf("Inspect second: %v", err)
	}
	if len(result2.Observed) == 0 {
		t.Error("second project should produce observed facts")
	}

	// Verify first project's artifacts unchanged
	var smCheck sourcemap.SourceMap
	if err := artStore.Read(artifactsDir, "sourcemap", &smCheck); err != nil {
		t.Fatalf("read first project sourcemap: %v", err)
	}
	if len(smCheck.Entries) != len(sm.Entries) {
		t.Errorf("first project sourcemap changed: had %d entries, now %d", len(sm.Entries), len(smCheck.Entries))
	}

	// 10. Verify no files written to fixture repo
	repoStatus, err := exec.Command("git", "-C", fixtureRepo, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if len(strings.TrimSpace(string(repoStatus))) > 0 {
		t.Errorf("fixture repo should have no changes, got: %s", repoStatus)
	}
}

func TestIntegration_DeterministicExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixtureRepo := createFixtureRepo(t)

	extractors := []inspect.Extractor{
		extract.NewFileTreeExtractor(),
		extract.NewGoModExtractor(),
		extract.NewValidationCommandsExtractor(),
	}

	cell := projectcell.Cell{
		Name:     "det-test",
		RepoPath: fixtureRepo,
		CellPath: t.TempDir(),
	}
	inferrer := infer.NewStubInferrer()
	inspector := inspect.NewInspector(extractors, inferrer)

	// Run inspect twice on unchanged repo
	result1, err := inspector.Inspect(context.Background(), cell)
	if err != nil {
		t.Fatalf("Inspect 1: %v", err)
	}
	result2, err := inspector.Inspect(context.Background(), cell)
	if err != nil {
		t.Fatalf("Inspect 2: %v", err)
	}

	// Observed facts should be identical count
	if len(result1.Observed) != len(result2.Observed) {
		t.Errorf("observed fact count differs: %d vs %d", len(result1.Observed), len(result2.Observed))
	}

	// Build source maps from both and compare
	sm1 := sourcemap.BuildFromFacts(result1.Observed)
	sm2 := sourcemap.BuildFromFacts(result2.Observed)
	if len(sm1.Entries) != len(sm2.Entries) {
		t.Errorf("source map entry count differs: %d vs %d", len(sm1.Entries), len(sm2.Entries))
	}
}

func TestIntegration_RelevanceBeforeBudgeting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	workspaceDir := t.TempDir()
	t.Setenv("GROMIT_HOME", workspaceDir)

	artStore := artifact.NewJSONStore()
	artifactsDir := filepath.Join(workspaceDir, "artifacts")
	os.MkdirAll(artifactsDir, 0o755)

	// Write architecture with multiple modules
	arch := architecture.New()
	arch.AddModule(architecture.Module{Name: "internal/auth", Description: "Auth module", Language: "go"})
	arch.AddModule(architecture.Module{Name: "internal/billing", Description: "Billing module", Language: "go"})
	artStore.Write(artifactsDir, "architecture", arch)

	doc := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			doctrine.NewRule("r1", "Rule one with some text", "all"),
			doctrine.NewRule("r2", "Rule two with more text", "all"),
		},
	}
	artStore.Write(artifactsDir, "doctrine", doc)

	cell := projectcell.Cell{Name: "budget-test", CellPath: workspaceDir}
	wrapper := &testArtifactStore{store: artStore, artifactsDir: artifactsDir}
	compiler := contextpkt.NewCompiler(wrapper)

	// Compile without budget
	unbounded, err := compiler.Compile(context.Background(), cell, contextpkt.LevelSpec, contextpkt.CompileOpts{
		SpecPath: "specs/001.md",
	})
	if err != nil {
		t.Fatalf("Compile unbounded: %v", err)
	}

	// Compile with small budget
	bounded, err := compiler.Compile(context.Background(), cell, contextpkt.LevelSpec, contextpkt.CompileOpts{
		SpecPath:    "specs/001.md",
		TokenBudget: 20,
	})
	if err != nil {
		t.Fatalf("Compile bounded: %v", err)
	}

	// Bounded should have fewer or equal tokens
	if bounded.TokenCount > unbounded.TokenCount {
		t.Errorf("bounded (%d) > unbounded (%d)", bounded.TokenCount, unbounded.TokenCount)
	}
	if bounded.TokenCount > 20 {
		t.Errorf("bounded token count %d exceeds budget 20", bounded.TokenCount)
	}
}

func createFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}

	// Configure git for commits
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Write files
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`,
		"internal/auth/auth.go": `package auth

type Service struct{}

func NewService() *Service {
	return &Service{}
}
`,
		"go.mod": `module github.com/example/fixture-app

go 1.22

require github.com/spf13/cobra v1.8.0
`,
		"Makefile": `.PHONY: test lint build

test:
	go test ./...

lint:
	go vet ./...

build:
	go build -o bin/app .
`,
	}

	for path, content := range files {
		full := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}

	// Make initial commit
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial").Run()

	return dir
}

// testArtifactStore wraps artifact.JSONStore to read from a specific artifacts directory.
type testArtifactStore struct {
	store        *artifact.JSONStore
	artifactsDir string
}

func (a *testArtifactStore) Read(cellPath string, art string, dest any) error {
	return a.store.Read(a.artifactsDir, art, dest)
}

func (a *testArtifactStore) Write(cellPath string, art string, src any) error {
	return a.store.Write(a.artifactsDir, art, src)
}

func (a *testArtifactStore) Exists(cellPath string, art string) bool {
	return a.store.Exists(a.artifactsDir, art)
}
