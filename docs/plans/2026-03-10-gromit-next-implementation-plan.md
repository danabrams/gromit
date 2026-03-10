# Gromit Next — Spec 0001 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the external project cell, context compiler, and agent guide — the stable foundation that future execution loops consume.

**Architecture:** Workspace-level project memory system. Project cells live outside repos at `$GROMIT_HOME/projects/<name>/`. Inspection extracts observed facts deterministically, then infers higher-level knowledge via LLM. Context packets compile minimal relevant slices from project memory at project/spec/task levels.

**Tech Stack:** Go, Cobra CLI, JSON artifact storage, standard library I/O.

**Design doc:** `docs/plans/2026-03-10-gromit-next-project-cell-design.md`
**Agent guide:** `internal/next/AGENTS.md`
**Verification plan:** `docs/plans/2026-03-10-gromit-next-verification-plan.md`

---

## Phase 1: Foundation Types and Storage

### Task 1: Fact types and knowledge categories

**Files:**
- Create: `internal/next/fact/fact.go`
- Test: `internal/next/fact/fact_test.go`

**Step 1: Write the failing test**

```go
package fact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{Declared, "declared"},
		{Observed, "observed"},
		{Inferred, "inferred"},
	}
	for _, tt := range tests {
		if got := tt.cat.String(); got != tt.want {
			t.Errorf("Category(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestNewFact(t *testing.T) {
	f := New("test-001", Observed, "go.mod declares Go 1.22", "go-module-extractor")
	if f.ID != "test-001" {
		t.Errorf("ID = %q, want %q", f.ID, "test-001")
	}
	if f.Category != Observed {
		t.Errorf("Category = %v, want Observed", f.Category)
	}
	if f.Source != "go-module-extractor" {
		t.Errorf("Source = %q, want %q", f.Source, "go-module-extractor")
	}
	if f.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestCategory_JSONRoundTrip(t *testing.T) {
	f := New("test-001", Observed, "test content", "test-source")
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"observed"`) {
		t.Errorf("expected JSON to contain \"observed\", got %s", data)
	}

	var f2 Fact
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if f2.Category != Observed {
		t.Errorf("Category = %v, want Observed", f2.Category)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/fact/ -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package fact

import (
	"encoding/json"
	"fmt"
	"time"
)

type Category int

const (
	Declared Category = iota
	Observed
	Inferred
)

func (c Category) String() string {
	switch c {
	case Declared:
		return "declared"
	case Observed:
		return "observed"
	case Inferred:
		return "inferred"
	default:
		return "unknown"
	}
}

func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *Category) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "declared":
		*c = Declared
	case "observed":
		*c = Observed
	case "inferred":
		*c = Inferred
	default:
		return fmt.Errorf("unknown category: %q", s)
	}
	return nil
}

type Fact struct {
	ID        string    `json:"id"`
	Category  Category  `json:"category"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

func New(id string, cat Category, content string, source string) Fact {
	return Fact{
		ID:        id,
		Category:  cat,
		Content:   content,
		Source:    source,
		Timestamp: time.Now(),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/fact/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/fact/
git commit -m "feat(next): add fact types and knowledge categories"
```

---

### Task 2: Artifact storage interface and JSON implementation

**Files:**
- Create: `internal/next/artifact/artifact.go`
- Create: `internal/next/artifact/json_store.go`
- Test: `internal/next/artifact/artifact_test.go`

**Step 1: Write the failing test**

```go
package artifact

import (
	"path/filepath"
	"testing"
)

func TestJSONStore_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	src := sample{Name: "test", Count: 42}
	if err := store.Write(dir, "sample", src); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var dest sample
	if err := store.Read(dir, "sample", &dest); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if dest.Name != "test" || dest.Count != 42 {
		t.Errorf("Read = %+v, want {test 42}", dest)
	}
}

func TestJSONStore_Exists(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	if store.Exists(dir, "missing") {
		t.Error("Exists should return false for missing artifact")
	}

	if err := store.Write(dir, "present", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !store.Exists(dir, "present") {
		t.Error("Exists should return true after Write")
	}
}

func TestJSONStore_WritesCorrectPath(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	store.Write(dir, "architecture", map[string]string{})

	expected := filepath.Join(dir, "architecture.json")
	if !fileExists(expected) {
		t.Errorf("expected file at %s", expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/artifact/ -v`
Expected: FAIL

**Step 3: Write minimal implementation**

`artifact.go` — interface definition:

```go
package artifact

type Store interface {
	Read(cellPath string, artifact string, dest any) error
	Write(cellPath string, artifact string, src any) error
	Exists(cellPath string, artifact string) bool
}
```

`json_store.go` — JSON file implementation:

```go
package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type JSONStore struct{}

func NewJSONStore() *JSONStore {
	return &JSONStore{}
}

func (s *JSONStore) Read(cellPath string, artifact string, dest any) error {
	data, err := os.ReadFile(s.path(cellPath, artifact))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (s *JSONStore) Write(cellPath string, artifact string, src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	p := s.path(cellPath, artifact)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (s *JSONStore) Exists(cellPath string, artifact string) bool {
	_, err := os.Stat(s.path(cellPath, artifact))
	return err == nil
}

func (s *JSONStore) path(cellPath string, artifact string) string {
	return filepath.Join(cellPath, artifact+".json")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/artifact/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/artifact/
git commit -m "feat(next): add artifact storage interface and JSON implementation"
```

---

### Task 3: Workspace resolution

**Files:**
- Modify: `internal/next/workspace/workspace.go` (replace scaffold)
- Test: `internal/next/workspace/workspace_test.go`

**Step 1: Write the failing test**

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvResolver_GROMIT_HOME(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GROMIT_HOME", dir)
	t.Setenv("XDG_DATA_HOME", "")

	r := NewEnvResolver()
	root, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if string(root) != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestEnvResolver_XDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GROMIT_HOME", "")
	t.Setenv("XDG_DATA_HOME", dir)

	r := NewEnvResolver()
	root, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(dir, "gromit")
	if string(root) != want {
		t.Errorf("root = %q, want %q", root, want)
	}
}

func TestEnvResolver_Default(t *testing.T) {
	t.Setenv("GROMIT_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	home, _ := os.UserHomeDir()

	r := NewEnvResolver()
	root, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "gromit")
	if string(root) != want {
		t.Errorf("root = %q, want %q", root, want)
	}
}

func TestRoot_ProjectsDir(t *testing.T) {
	root := Root("/workspace")
	if got := root.ProjectsDir(); got != "/workspace/projects" {
		t.Errorf("ProjectsDir = %q, want %q", got, "/workspace/projects")
	}
}

func TestRoot_ProjectCell(t *testing.T) {
	root := Root("/workspace")
	if got := root.ProjectCell("myapp"); got != "/workspace/projects/myapp" {
		t.Errorf("ProjectCell = %q, want %q", got, "/workspace/projects/myapp")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/workspace/ -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Replace `workspace.go`:

```go
package workspace

import (
	"os"
	"path/filepath"
)

type Root string

func (r Root) ProjectsDir() string {
	return filepath.Join(string(r), "projects")
}

func (r Root) ProjectCell(name string) string {
	return filepath.Join(r.ProjectsDir(), name)
}

type Resolver interface {
	Resolve() (Root, error)
}

type EnvResolver struct{}

func NewEnvResolver() *EnvResolver {
	return &EnvResolver{}
}

func (r *EnvResolver) Resolve() (Root, error) {
	if v := os.Getenv("GROMIT_HOME"); v != "" {
		return Root(v), nil
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return Root(filepath.Join(v, "gromit")), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return Root(filepath.Join(home, ".local", "share", "gromit")), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/workspace/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/workspace/
git commit -m "feat(next): implement workspace resolution with GROMIT_HOME, XDG, and default"
```

---

### Task 4: Project cell store

**Files:**
- Modify: `internal/next/projectcell/projectcell.go` (replace scaffold)
- Test: `internal/next/projectcell/projectcell_test.go`

**Step 1: Write the failing test**

```go
package projectcell

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
}

func TestFSStore_CreateAndGet(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, err := store.Create("myapp", repoDir)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cell.Name != "myapp" {
		t.Errorf("Name = %q, want %q", cell.Name, "myapp")
	}
	if cell.RepoPath != repoDir {
		t.Errorf("RepoPath = %q, want %q", cell.RepoPath, repoDir)
	}

	got, err := store.Get("myapp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "myapp" || got.RepoPath != repoDir {
		t.Errorf("Get returned %+v", got)
	}
}

func TestFSStore_CreateDuplicate(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	if _, err := store.Create("myapp", repoDir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := store.Create("myapp", repoDir); err == nil {
		t.Error("expected error on duplicate create")
	}
}

func TestFSStore_CreateNonGitRepo(t *testing.T) {
	workspace := t.TempDir()
	notARepo := t.TempDir()

	store := NewFSStore(filepath.Join(workspace, "projects"))
	if _, err := store.Create("myapp", notARepo); err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestFSStore_List(t *testing.T) {
	workspace := t.TempDir()
	store := NewFSStore(filepath.Join(workspace, "projects"))

	repo1 := t.TempDir()
	initGitRepo(t, repo1)
	repo2 := t.TempDir()
	initGitRepo(t, repo2)

	store.Create("alpha", repo1)
	store.Create("beta", repo2)

	cells, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("List returned %d cells, want 2", len(cells))
	}
}

func TestFSStore_Delete(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	store.Create("myapp", repoDir)

	if err := store.Delete("myapp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("myapp"); err == nil {
		t.Error("Get should fail after Delete")
	}
}

func TestFSStore_CreateBuildsDirectoryStructure(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, _ := store.Create("myapp", repoDir)

	for _, sub := range []string{"artifacts", "doctrine", "provenance", "guide"} {
		dir := filepath.Join(cell.CellPath, sub)
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("subdirectory %s missing: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/projectcell/ -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Replace `projectcell.go`:

```go
package projectcell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Cell struct {
	Name      string    `json:"name"`
	RepoPath  string    `json:"repo_path"`
	CreatedAt time.Time `json:"created_at"`
	CellPath  string    `json:"-"`
}

type Store interface {
	Create(name string, repoPath string) (Cell, error)
	Get(name string) (Cell, error)
	List() ([]Cell, error)
	Delete(name string) error
}

type FSStore struct {
	projectsDir string
}

func NewFSStore(projectsDir string) *FSStore {
	return &FSStore{projectsDir: projectsDir}
}

func (s *FSStore) Create(name string, repoPath string) (Cell, error) {
	cellDir := filepath.Join(s.projectsDir, name)
	if _, err := os.Stat(cellDir); err == nil {
		return Cell{}, fmt.Errorf("project %q already exists", name)
	}
	if !isGitRepo(repoPath) {
		return Cell{}, fmt.Errorf("%q is not a git repository", repoPath)
	}

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return Cell{}, err
	}

	cell := Cell{
		Name:      name,
		RepoPath:  abs,
		CreatedAt: time.Now(),
		CellPath:  cellDir,
	}

	for _, sub := range []string{"artifacts", "doctrine", "provenance", "guide"} {
		if err := os.MkdirAll(filepath.Join(cellDir, sub), 0o755); err != nil {
			return Cell{}, err
		}
	}

	data, err := json.MarshalIndent(cell, "", "  ")
	if err != nil {
		return Cell{}, err
	}
	return cell, os.WriteFile(filepath.Join(cellDir, "project.json"), data, 0o644)
}

func (s *FSStore) Get(name string) (Cell, error) {
	cellDir := filepath.Join(s.projectsDir, name)
	data, err := os.ReadFile(filepath.Join(cellDir, "project.json"))
	if err != nil {
		return Cell{}, fmt.Errorf("project %q not found: %w", name, err)
	}
	var cell Cell
	if err := json.Unmarshal(data, &cell); err != nil {
		return Cell{}, err
	}
	cell.CellPath = cellDir
	return cell, nil
}

func (s *FSStore) List() ([]Cell, error) {
	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cells []Cell
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cell, err := s.Get(e.Name())
		if err != nil {
			continue
		}
		cells = append(cells, cell)
	}
	return cells, nil
}

func (s *FSStore) Delete(name string) error {
	cellDir := filepath.Join(s.projectsDir, name)
	return os.RemoveAll(cellDir)
}

func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/projectcell/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/projectcell/
git commit -m "feat(next): implement filesystem project cell store"
```

---

## Phase 2: Provenance and Doctrine

### Task 5: Provenance tracker

**Files:**
- Modify: `internal/next/provenance/provenance.go` (replace scaffold)
- Test: `internal/next/provenance/provenance_test.go`

**Step 1: Write the failing test**

```go
package provenance

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFSTracker_RecordAndCheck(t *testing.T) {
	dir := t.TempDir()
	tracker := NewFSTracker(filepath.Join(dir, "provenance.json"))

	rec := Record{
		FactID:    "arch-001",
		Artifact:  "architecture",
		Category:  "observed",
		GitSHA:    "abc123",
		Timestamp: time.Now(),
		Extractor: "go-module",
		InputHash: "sha256:deadbeef",
	}
	if err := tracker.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := tracker.Check("architecture")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.GitSHA != "abc123" {
		t.Errorf("GitSHA = %q, want %q", got.GitSHA, "abc123")
	}
}

func TestFSTracker_IsFresh(t *testing.T) {
	dir := t.TempDir()
	tracker := NewFSTracker(filepath.Join(dir, "provenance.json"))

	tracker.Record(Record{
		Artifact: "architecture",
		GitSHA:   "abc123",
	})

	fresh, err := tracker.IsFresh("architecture", "abc123")
	if err != nil {
		t.Fatalf("IsFresh: %v", err)
	}
	if !fresh {
		t.Error("should be fresh with same SHA")
	}

	fresh, _ = tracker.IsFresh("architecture", "def456")
	if fresh {
		t.Error("should not be fresh with different SHA")
	}
}

func TestFSTracker_CheckMissing(t *testing.T) {
	dir := t.TempDir()
	tracker := NewFSTracker(filepath.Join(dir, "provenance.json"))

	_, err := tracker.Check("nonexistent")
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/provenance/ -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Replace `provenance.go`:

```go
package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Record struct {
	FactID    string    `json:"fact_id"`
	Artifact  string    `json:"artifact"`
	Category  string    `json:"category"`
	GitSHA    string    `json:"git_sha"`
	Timestamp time.Time `json:"timestamp"`
	Extractor string    `json:"extractor"`
	InputHash string    `json:"input_hash"`
}

type Tracker interface {
	Record(rec Record) error
	Check(artifactName string) (Record, error)
	IsFresh(artifactName string, currentSHA string) (bool, error)
}

type FSTracker struct {
	path string
}

func NewFSTracker(path string) *FSTracker {
	return &FSTracker{path: path}
}

func (t *FSTracker) Record(rec Record) error {
	records, _ := t.load()
	records[rec.Artifact] = rec
	return t.save(records)
}

func (t *FSTracker) Check(artifactName string) (Record, error) {
	records, err := t.load()
	if err != nil {
		return Record{}, err
	}
	rec, ok := records[artifactName]
	if !ok {
		return Record{}, fmt.Errorf("no provenance for artifact %q", artifactName)
	}
	return rec, nil
}

func (t *FSTracker) IsFresh(artifactName string, currentSHA string) (bool, error) {
	rec, err := t.Check(artifactName)
	if err != nil {
		return false, err
	}
	return rec.GitSHA == currentSHA, nil
}

func (t *FSTracker) load() (map[string]Record, error) {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]Record), nil
		}
		return nil, err
	}
	var records map[string]Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (t *FSTracker) save(records map[string]Record) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.path, data, 0o644)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/provenance/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/provenance/
git commit -m "feat(next): implement provenance tracker with artifact-level freshness"
```

---

### Task 6: Doctrine store

**Files:**
- Modify: `internal/next/doctrine/doctrine.go` (replace scaffold)
- Test: `internal/next/doctrine/doctrine_test.go`

**Step 1: Write the failing test**

```go
package doctrine

import (
	"path/filepath"
	"testing"
)

func TestFSStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFSStore()

	d := Doctrine{
		Rules: []Rule{
			{ID: "arch-001", Summary: "Use hexagonal architecture", Scope: "architecture"},
		},
	}

	if err := store.Save(filepath.Join(dir, "doctrine"), d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load(filepath.Join(dir, "doctrine"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Rules) != 1 || got.Rules[0].ID != "arch-001" {
		t.Errorf("Load = %+v, want 1 rule with ID arch-001", got)
	}
}

func TestFSStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewFSStore()

	got, err := store.Load(filepath.Join(dir, "doctrine"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(got.Rules))
	}
}

func TestRule_SourceAlwaysDeclared(t *testing.T) {
	r := NewRule("test-001", "Test rule", "testing")
	if r.Source != "declared" {
		t.Errorf("Source = %q, want %q", r.Source, "declared")
	}
	if r.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/doctrine/ -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Replace `doctrine.go`:

```go
package doctrine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Rule struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Scope     string    `json:"scope"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func NewRule(id string, summary string, scope string) Rule {
	return Rule{
		ID:        id,
		Summary:   summary,
		Scope:     scope,
		Source:    "declared",
		CreatedAt: time.Now(),
	}
}

type Doctrine struct {
	Rules []Rule `json:"rules"`
}

type Store interface {
	Save(doctrineDir string, d Doctrine) error
	Load(doctrineDir string) (Doctrine, error)
}

type FSStore struct{}

func NewFSStore() *FSStore {
	return &FSStore{}
}

func (s *FSStore) Save(doctrineDir string, d Doctrine) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(doctrineDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(doctrineDir, "rules.json"), data, 0o644)
}

func (s *FSStore) Load(doctrineDir string) (Doctrine, error) {
	data, err := os.ReadFile(filepath.Join(doctrineDir, "rules.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Doctrine{Rules: []Rule{}}, nil
		}
		return Doctrine{}, err
	}
	var d Doctrine
	if err := json.Unmarshal(data, &d); err != nil {
		return Doctrine{}, err
	}
	return d, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/doctrine/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/doctrine/
git commit -m "feat(next): implement doctrine store for declared knowledge"
```

---

## Phase 3: Inspection

### Task 7: Extractor interface and file tree extractor

**Files:**
- Create: `internal/next/extract/extract.go`
- Create: `internal/next/extract/filetree.go`
- Create: `internal/next/extract/testdata/sample-repo/` (fixture)
- Test: `internal/next/extract/filetree_test.go`

**Step 1: Write the failing test**

```go
package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func setupFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"main.go":           "package main",
		"internal/auth/auth.go": "package auth",
		"go.mod":            "module example.com/test",
		"README.md":         "# Test",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	return dir
}

func TestFileTreeExtractor_Extract(t *testing.T) {
	repo := setupFixtureRepo(t)
	ext := NewFileTreeExtractor()

	facts, err := ext.Extract(repo)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("expected at least one fact")
	}

	// All facts should be observed
	for _, f := range facts {
		if f.Category.String() != "observed" {
			t.Errorf("fact %q has category %v, want observed", f.ID, f.Category)
		}
	}
}

func TestFileTreeExtractor_Name(t *testing.T) {
	ext := NewFileTreeExtractor()
	if ext.Name() != "file-tree" {
		t.Errorf("Name = %q, want %q", ext.Name(), "file-tree")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/extract/ -v`
Expected: FAIL

**Step 3: Implement**

- `extract.go`: `Extractor` interface with `Name() string` and `Extract(repoPath string) ([]fact.Fact, error)`
- `filetree.go`: walks the directory tree, skips `.git/` and common ignores, produces one fact per file with relative path, language (from extension), and line count

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/extract/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/extract/
git commit -m "feat(next): add extractor interface and file tree extractor"
```

---

### Task 8: Go module extractor

**Files:**
- Create: `internal/next/extract/gomod.go`
- Test: `internal/next/extract/gomod_test.go`

**Step 1: Write the failing test**

```go
package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoModExtractor_Extract(t *testing.T) {
	dir := t.TempDir()
	gomod := `module github.com/example/myapp

go 1.22

require (
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.9.0
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	ext := NewGoModExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) < 3 {
		t.Fatalf("expected at least 3 facts (module, go version, deps), got %d", len(facts))
	}

	// All facts should be observed
	for _, f := range facts {
		if f.Category.String() != "observed" {
			t.Errorf("fact %q has category %v, want observed", f.ID, f.Category)
		}
		if f.Source != "go-module" {
			t.Errorf("fact %q has source %q, want %q", f.ID, f.Source, "go-module")
		}
	}
}

func TestGoModExtractor_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	ext := NewGoModExtractor()

	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract should not error on missing go.mod: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for missing go.mod, got %d", len(facts))
	}
}

func TestGoModExtractor_Name(t *testing.T) {
	ext := NewGoModExtractor()
	if ext.Name() != "go-module" {
		t.Errorf("Name = %q, want %q", ext.Name(), "go-module")
	}
}
```

**Step 2-5:** Follow standard TDD flow. Extracts module path, Go version, and direct dependencies from `go.mod`. Produces observed facts.

**Commit:** `feat(next): add go.mod extractor`

---

### Task 9: Makefile/CI extractor

**Files:**
- Create: `internal/next/extract/validation_commands.go`
- Test: `internal/next/extract/validation_commands_test.go`

**Step 1: Write the failing test**

```go
package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidationCommandsExtractor_Makefile(t *testing.T) {
	dir := t.TempDir()
	makefile := `.PHONY: test lint build

test:
	go test ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/app ./cmd/app
`
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644)

	ext := NewValidationCommandsExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) < 3 {
		t.Fatalf("expected at least 3 facts (test, lint, build), got %d", len(facts))
	}

	for _, f := range facts {
		if f.Category.String() != "observed" {
			t.Errorf("fact %q has category %v, want observed", f.ID, f.Category)
		}
	}
}

func TestValidationCommandsExtractor_CIWorkflow(t *testing.T) {
	dir := t.TempDir()
	workflowDir := filepath.Join(dir, ".github", "workflows")
	os.MkdirAll(workflowDir, 0o755)

	workflow := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go test ./...
      - run: golangci-lint run
`
	os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte(workflow), 0o644)

	ext := NewValidationCommandsExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) < 2 {
		t.Fatalf("expected at least 2 facts from CI workflow, got %d", len(facts))
	}
}

func TestValidationCommandsExtractor_NoFiles(t *testing.T) {
	dir := t.TempDir()
	ext := NewValidationCommandsExtractor()

	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract should not error on missing files: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
}

func TestValidationCommandsExtractor_Name(t *testing.T) {
	ext := NewValidationCommandsExtractor()
	if ext.Name() != "validation-commands" {
		t.Errorf("Name = %q, want %q", ext.Name(), "validation-commands")
	}
}
```

**Step 2-5:** Follow standard TDD flow. Extracts test/lint/build commands from `Makefile` targets and `.github/workflows/*.yml` `run:` steps. Produces observed facts tagged with source file.

**Commit:** `feat(next): add validation commands extractor`

---

### Task 10: LLM inference interface and stub

**Files:**
- Create: `internal/next/infer/infer.go`
- Test: `internal/next/infer/infer_test.go`

Define the `Inferrer` interface. Create a `StubInferrer` that returns empty inferred facts (for testing and initial wiring). The real LLM implementation is a later task — this task establishes the contract.

**Step 1: Write the failing test**

```go
package infer

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestStubInferrer_ReturnsEmpty(t *testing.T) {
	stub := NewStubInferrer()
	observed := []fact.Fact{
		fact.New("f1", fact.Observed, "go.mod exists", "gomod"),
	}

	inferred, err := stub.Infer(context.Background(), observed)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(inferred) != 0 {
		t.Errorf("expected empty inferred facts, got %d", len(inferred))
	}
}

func TestStubInferrer_ImplementsInferrer(t *testing.T) {
	var _ Inferrer = (*StubInferrer)(nil)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/infer/ -v`
Expected: FAIL

**Step 3: Implement**

```go
package infer

import (
	"context"

	"github.com/danabrams/gromit/internal/next/fact"
)

type Inferrer interface {
	Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error)
}

type StubInferrer struct{}

func NewStubInferrer() *StubInferrer {
	return &StubInferrer{}
}

func (s *StubInferrer) Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error) {
	return []fact.Fact{}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/infer/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/infer/
git commit -m "feat(next): add inferrer interface and stub implementation"
```

---

### Task 11: Inspection orchestrator

**Files:**
- Modify: `internal/next/inspect/inspect.go` (replace scaffold)
- Modify: `internal/next/inspect/types.go` (replace scaffold)
- Test: `internal/next/inspect/inspect_test.go`

**Step 1: Write the failing test**

```go
package inspect

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/next/projectcell"
)

type mockExtractor struct {
	name  string
	facts []fact.Fact
}

func (m *mockExtractor) Name() string                          { return m.name }
func (m *mockExtractor) Extract(repoPath string) ([]fact.Fact, error) { return m.facts, nil }

type mockInferrer struct {
	facts []fact.Fact
}

func (m *mockInferrer) Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error) {
	return m.facts, nil
}

func TestDefaultInspector_Inspect(t *testing.T) {
	cell := projectcell.Cell{
		Name:     "test",
		RepoPath: t.TempDir(),
		CellPath: t.TempDir(),
	}

	observed := []fact.Fact{fact.New("f1", fact.Observed, "go.mod exists", "gomod")}
	inferred := []fact.Fact{fact.New("f2", fact.Inferred, "uses DDD", "llm")}

	inspector := NewInspector(
		[]Extractor{&mockExtractor{name: "gomod", facts: observed}},
		&mockInferrer{facts: inferred},
	)

	result, err := inspector.Inspect(context.Background(), cell)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(result.Observed) != 1 || len(result.Inferred) != 1 {
		t.Errorf("got %d observed, %d inferred; want 1, 1", len(result.Observed), len(result.Inferred))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/inspect/ -v`
Expected: FAIL

**Step 3: Implement**

- `types.go`: `Result` struct with `Observed` and `Inferred` fact slices. `Extractor` interface (re-exported from extract package or defined locally).
- `inspect.go`: `Inspector` interface. `DefaultInspector` struct that runs extractors in sequence, collects observed facts, passes them to the inferrer, returns combined result.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/inspect/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/inspect/
git commit -m "feat(next): implement inspection orchestrator with extract-then-infer pipeline"
```

---

## Phase 4: Agent Guide

### Task 12: Source map builder

**Files:**
- Modify: `internal/next/sourcemap/sourcemap.go` (replace scaffold)
- Test: `internal/next/sourcemap/sourcemap_test.go`

Define `SourceMap` and `Entry` types. Implement `BuildFromFacts(facts []fact.Fact) SourceMap` that filters file-tree facts and structures them into a source map. Test with sample facts.

**Commit:** `feat(next): implement source map builder from extracted facts`

---

### Task 13: Architecture types

**Files:**
- Create: `internal/next/architecture/architecture.go`
- Test: `internal/next/architecture/architecture_test.go`

**Step 1: Write the failing test**

```go
package architecture

import "testing"

func TestModule_String(t *testing.T) {
	m := Module{
		Name:        "internal/auth",
		Description: "Authentication and authorization",
		Language:    "go",
	}
	if m.Name != "internal/auth" {
		t.Errorf("Name = %q, want %q", m.Name, "internal/auth")
	}
}

func TestDependency_Directions(t *testing.T) {
	d := Dependency{
		From: "internal/auth",
		To:   "internal/config",
		Kind: "import",
	}
	if d.From != "internal/auth" || d.To != "internal/config" {
		t.Errorf("unexpected dependency: %+v", d)
	}
}

func TestArchitecture_AddModule(t *testing.T) {
	arch := New()
	arch.AddModule(Module{Name: "cmd/api", Description: "HTTP entrypoint", Language: "go"})
	arch.AddModule(Module{Name: "internal/core", Description: "Domain logic", Language: "go"})

	if len(arch.Modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(arch.Modules))
	}
}

func TestArchitecture_AddDependency(t *testing.T) {
	arch := New()
	arch.AddDependency(Dependency{From: "cmd/api", To: "internal/core", Kind: "import"})

	if len(arch.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(arch.Dependencies))
	}
}

func TestNew_InitializesEmptySlices(t *testing.T) {
	arch := New()
	if arch.Modules == nil {
		t.Error("Modules should be initialized, not nil")
	}
	if arch.Dependencies == nil {
		t.Error("Dependencies should be initialized, not nil")
	}
	if arch.Components == nil {
		t.Error("Components should be initialized, not nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/architecture/ -v`
Expected: FAIL

**Step 3: Implement**

```go
package architecture

// Module represents a code module boundary within the project.
type Module struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
}

// Dependency represents a directional relationship between modules.
type Dependency struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // "import", "call", "config", etc.
}

// Component describes a high-level architectural component.
type Component struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Modules     []string `json:"modules"`
}

// Architecture holds the discovered module boundaries, dependency
// relationships, and component descriptions for a project. Consumed by
// the guide renderer and context compiler.
type Architecture struct {
	Modules      []Module     `json:"modules"`
	Dependencies []Dependency `json:"dependencies"`
	Components   []Component  `json:"components"`
}

// New returns an Architecture with all slices initialized to empty.
func New() Architecture {
	return Architecture{
		Modules:      []Module{},
		Dependencies: []Dependency{},
		Components:   []Component{},
	}
}

func (a *Architecture) AddModule(m Module) {
	a.Modules = append(a.Modules, m)
}

func (a *Architecture) AddDependency(d Dependency) {
	a.Dependencies = append(a.Dependencies, d)
}

func (a *Architecture) AddComponent(c Component) {
	a.Components = append(a.Components, c)
}

func (a *Architecture) NormalizeNilFields() {
	if a.Modules == nil {
		a.Modules = []Module{}
	}
	if a.Dependencies == nil {
		a.Dependencies = []Dependency{}
	}
	if a.Components == nil {
		a.Components = []Component{}
	}
}
```

Per project convention (CLAUDE.md), the exported `NormalizeNilFields()` method maps nil slices to empty values. Call this in JSON deserialization paths.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/architecture/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/architecture/
git commit -m "feat(next): add architecture types for module boundaries and dependencies"
```

---

### Task 14: Validation types

**Files:**
- Create: `internal/next/validation/validation.go`
- Test: `internal/next/validation/validation_test.go`

**Step 1: Write the failing test**

```go
package validation

import "testing"

func TestCommand_String(t *testing.T) {
	cmd := Command{
		Name:    "go-test",
		Kind:    Test,
		Run:     "go test ./...",
		Source:  "Makefile",
	}
	if cmd.Name != "go-test" {
		t.Errorf("Name = %q, want %q", cmd.Name, "go-test")
	}
	if cmd.Kind != Test {
		t.Errorf("Kind = %v, want Test", cmd.Kind)
	}
}

func TestKind_String(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{Test, "test"},
		{Lint, "lint"},
		{Build, "build"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestCommandSet_Add(t *testing.T) {
	cs := NewCommandSet()
	cs.Add(Command{Name: "test", Kind: Test, Run: "go test ./...", Source: "Makefile"})
	cs.Add(Command{Name: "lint", Kind: Lint, Run: "golangci-lint run", Source: "Makefile"})

	if len(cs.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(cs.Commands))
	}
}

func TestCommandSet_ByKind(t *testing.T) {
	cs := NewCommandSet()
	cs.Add(Command{Name: "unit", Kind: Test, Run: "go test ./...", Source: "Makefile"})
	cs.Add(Command{Name: "lint", Kind: Lint, Run: "golangci-lint run", Source: "Makefile"})
	cs.Add(Command{Name: "integ", Kind: Test, Run: "go test -tags=integration ./...", Source: "CI"})

	tests := cs.ByKind(Test)
	if len(tests) != 2 {
		t.Errorf("expected 2 test commands, got %d", len(tests))
	}
}

func TestNewCommandSet_InitializesEmpty(t *testing.T) {
	cs := NewCommandSet()
	if cs.Commands == nil {
		t.Error("Commands should be initialized, not nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/validation/ -v`
Expected: FAIL

**Step 3: Implement**

```go
package validation

// Kind categorizes what a validation command checks.
type Kind int

const (
	Test  Kind = iota
	Lint
	Build
)

func (k Kind) String() string {
	switch k {
	case Test:
		return "test"
	case Lint:
		return "lint"
	case Build:
		return "build"
	default:
		return "unknown"
	}
}

// Command represents a validation command to run — not a pass/fail result,
// but the specification of what to execute. Parsed from Makefiles, CI configs,
// and other extractors.
type Command struct {
	Name   string `json:"name"`
	Kind   Kind   `json:"kind"`
	Run    string `json:"run"`
	Source string `json:"source"` // file the command was extracted from
}

// CommandSet holds a collection of validation commands for a project.
type CommandSet struct {
	Commands []Command `json:"commands"`
}

// NewCommandSet returns a CommandSet with an initialized empty slice.
func NewCommandSet() CommandSet {
	return CommandSet{Commands: []Command{}}
}

func (cs *CommandSet) Add(cmd Command) {
	cs.Commands = append(cs.Commands, cmd)
}

func (cs *CommandSet) NormalizeNilFields() {
	if cs.Commands == nil {
		cs.Commands = []Command{}
	}
}

// ByKind returns all commands matching the given kind.
func (cs *CommandSet) ByKind(kind Kind) []Command {
	var result []Command
	for _, cmd := range cs.Commands {
		if cmd.Kind == kind {
			result = append(result, cmd)
		}
	}
	return result
}
```

Per project convention (CLAUDE.md), the exported `NormalizeNilFields()` method on `CommandSet` maps nil slices to empty values. Call this in JSON deserialization paths.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/validation/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/validation/
git commit -m "feat(next): add validation command types for test, lint, and build"
```

---

### Task 15: Agent guide renderer

**Files:**
- Modify: `internal/next/guide/guide.go` (replace scaffold)
- Test: `internal/next/guide/guide_test.go`

**Step 1: Write the failing test**

```go
package guide

import (
	"strings"
	"testing"
)

func TestMarkdownRenderer_Render(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "payments-api",
		Glossary: []GlossaryEntry{
			{Term: "PCI", Definition: "Payment Card Industry compliance standard"},
		},
	}

	output, err := r.Render(input)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	md := string(output)
	if !strings.Contains(md, "# payments-api") {
		t.Error("missing project heading")
	}
	if !strings.Contains(md, "PCI") {
		t.Error("missing glossary entry")
	}
}

func TestMarkdownRenderer_OmitsEmptySections(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "minimal",
	}

	output, _ := r.Render(input)
	md := string(output)
	if strings.Contains(md, "## Glossary") {
		t.Error("should omit empty Glossary section")
	}
	if strings.Contains(md, "## Risky Areas") {
		t.Error("should omit empty Risky Areas section")
	}
}
```

**Step 2-5:** Follow standard TDD flow. The renderer writes markdown with structured sections. Each section is rendered only if the corresponding input is non-empty.

**Commit:** `feat(next): implement agent guide markdown renderer`

---

## Phase 5: Context Compilation

### Task 16: Context compiler — project level

**Note:** Delete the existing scaffold at `internal/next/context/` and create `internal/next/contextpkt/` as the replacement. The package was renamed to avoid shadowing Go's standard library `context` package.

**Files:**
- Modify: `internal/next/contextpkt/context.go` (replace scaffold)
- Test: `internal/next/contextpkt/context_test.go`

**Step 1: Write the failing test**

```go
package contextpkt

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/projectcell"
)

type mockArtifactStore struct {
	arch     architecture.Architecture
	doctrine doctrine.Doctrine
}

func (m *mockArtifactStore) Read(cellPath string, artifact string, dest any) error {
	switch d := dest.(type) {
	case *architecture.Architecture:
		*d = m.arch
	case *doctrine.Doctrine:
		*d = m.doctrine
	default:
		return fmt.Errorf("mock: unsupported type %T", dest)
	}
	return nil
}
func (m *mockArtifactStore) Write(cellPath string, artifact string, src any) error { return nil }
func (m *mockArtifactStore) Exists(cellPath string, artifact string) bool           { return true }

func TestCompiler_ProjectLevel(t *testing.T) {
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "arch-001", Summary: "Use hexagonal architecture", Scope: "architecture"},
			},
		},
	}

	cell := projectcell.Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(packet.Sections) == 0 {
		t.Fatal("expected at least one section in the packet")
	}

	sectionNames := make(map[string]bool)
	for _, s := range packet.Sections {
		sectionNames[s.Name] = true
	}
	for _, want := range []string{"architecture", "doctrine"} {
		if !sectionNames[want] {
			t.Errorf("missing section %q in packet", want)
		}
	}
	if packet.TokenCount == 0 {
		t.Error("token count should be populated")
	}
}

func TestCompiler_TokenBudget(t *testing.T) {
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module with a long description that takes up tokens", Language: "go"},
				{Name: "internal/billing", Description: "Billing module with another long description", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "r1", Summary: "Rule one", Scope: "all"},
				{ID: "r2", Summary: "Rule two", Scope: "all"},
			},
		},
	}

	cell := projectcell.Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	// Compile with a very small budget
	packet, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{TokenBudget: 50})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if packet.TokenCount > 50 {
		t.Errorf("packet token count %d exceeds budget 50", packet.TokenCount)
	}
}
```

Implement `DefaultCompiler` with a `Compile` method. For project level: load architecture, doctrine, glossary, and validation artifacts. Build sections with token estimates. Apply budget if set.

**Commit:** `feat(next): implement context compiler for project level`

---

### Task 17: Context compiler — spec and task levels

**Files:**
- Modify: `internal/next/contextpkt/context.go`
- Test: `internal/next/contextpkt/context_test.go` (add tests)

**Step 1: Write the failing tests**

```go
// Add to contextpkt/context_test.go

func TestCompiler_SpecLevel(t *testing.T) {
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "r1", Summary: "Use hexagonal architecture", Scope: "architecture"},
			},
		},
	}

	cell := projectcell.Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelSpec, CompileOpts{
		SpecPath: "specs/001-auth-redesign.md",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(packet.Sections) == 0 {
		t.Fatal("expected at least one section")
	}

	// Spec level should include project context plus spec-specific content
	sectionNames := make(map[string]bool)
	for _, s := range packet.Sections {
		sectionNames[s.Name] = true
	}
	if !sectionNames["architecture"] {
		t.Error("spec level should include architecture section from project")
	}
	if !sectionNames["spec-text"] {
		t.Error("spec level should include spec-text section")
	}
}

func TestCompiler_TaskLevel(t *testing.T) {
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "r1", Summary: "TDD required", Scope: "testing"},
			},
		},
	}

	cell := projectcell.Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelTask, CompileOpts{
		SpecPath: "specs/001-auth-redesign.md",
		TaskID:   "task-3",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(packet.Sections) == 0 {
		t.Fatal("expected at least one section")
	}

	// Task level should include project + spec context plus task scope
	sectionNames := make(map[string]bool)
	for _, s := range packet.Sections {
		sectionNames[s.Name] = true
	}
	if !sectionNames["doctrine"] {
		t.Error("task level should include doctrine section")
	}
	if !sectionNames["proof-requirements"] {
		t.Error("task level should include proof-requirements section")
	}
}

func TestCompiler_SpecLevelMissingSpecPath(t *testing.T) {
	store := &mockArtifactStore{}
	cell := projectcell.Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	_, err := compiler.Compile(context.Background(), cell, LevelSpec, CompileOpts{})
	if err == nil {
		t.Error("expected error when spec path is empty for spec level")
	}
}

func TestCompiler_TaskLevelMissingTaskID(t *testing.T) {
	store := &mockArtifactStore{}
	cell := projectcell.Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	_, err := compiler.Compile(context.Background(), cell, LevelTask, CompileOpts{
		SpecPath: "specs/001.md",
	})
	if err == nil {
		t.Error("expected error when task ID is empty for task level")
	}
}
```

**Step 2-5:** Follow standard TDD flow. Add spec-level compilation (reads spec file, selects relevant project facts) and task-level compilation (reads task scope, selects relevant project + spec facts, includes proof requirements). The `Compile` method must validate required fields per level: spec level requires `SpecPath`, task level requires both `SpecPath` and `TaskID`.

**Commit:** `feat(next): add spec and task level context compilation`

---

## Phase 6: CLI Wiring

### Task 18: CLI skeleton — main and project commands

**Files:**
- Create: `cmd/gromit-next/main.go`
- Create: `cmd/gromit-next/project.go`

Wire Cobra commands: `project attach`, `project inspect`, `project guide`, `project list`. Each command resolves the workspace, loads the project cell store, and calls the appropriate internal package. Follow the legacy `cmd/gromit/main.go` Cobra patterns.

**Commit:** `feat(next): add gromit-next CLI with project commands`

---

### Task 19: CLI — context build command

**Files:**
- Create: `cmd/gromit-next/context.go`

Wire `context build <name> --level project|spec|task --spec <path> --task <id>`. Validate required flags per level.

**Commit:** `feat(next): add context build command to gromit-next CLI`

---

### Task 20: End-to-end integration test

**Files:**
- Create: `internal/next/integration_test.go`

Test the full flow: create workspace in temp dir → attach a fixture repo → run inspect (with stub inferrer) → render guide → compile project context. Verify all artifacts exist and the guide contains expected sections.

**Commit:** `test(next): add end-to-end integration test for project cell flow`

---

## Task Dependency Graph

```
Task 1 (fact types)
  ├─→ Task 2 (artifact store)
  ├─→ Task 5 (provenance)
  ├─→ Task 6 (doctrine)
  ├─→ Task 7 (file tree extractor)
  │     ├─→ Task 8 (gomod extractor)
  │     └─→ Task 9 (CI extractor)
  ├─→ Task 10 (inferrer stub)
  │     └─→ Task 4 + Task 10 ─→ Task 11 (inspect orchestrator)
  ├─→ Task 12 (source map)
  ├─→ Task 13 (architecture types)
  └─→ Task 14 (validation types)

Task 3 (workspace) ─→ Task 4 (project cell)

Task 6 + Task 12 + Task 13 + Task 14 ─→ Task 15 (guide renderer)

Task 1 + Task 2 + Task 4 + Task 5 + Task 6 + Task 13 + Task 14 → Task 16 (context compiler - project)
Task 16 ─→ Task 17 (context compiler - spec/task)

Task 4 + Task 11 + Task 15 + Task 17 ─→ Task 18 (CLI project)
Task 18 ─→ Task 19 (CLI context)
Task 19 ─→ Task 20 (integration test)
```

Tasks 1-6 and Task 3 can be parallelized. Tasks 7-9, 12-14 can be parallelized after Task 1.
