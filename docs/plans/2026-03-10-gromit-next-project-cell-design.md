# Design: External Project Cell, Context Compiler, and Agent Guide

**Date:** 2026-03-10
**Spec:** 0001
**Status:** Draft

---

## Problem

The current Gromit prototype couples state to individual repos and spreads project context across repo-local files, prompt fragments, and execution behavior. Multi-project use is awkward, cross-project contamination is possible, and the loop carries too much knowledge about project structure.

The new system separates project memory from the execution loop and from the repos themselves.

---

## Architecture Overview

```
$GROMIT_HOME/
  projects/
    payments-api/
      project.json          # project config (name, repo path, alias)
      artifacts/
        architecture.json   # observed + inferred architecture
        sourcemap.json      # file inventory with metadata
        glossary.json       # domain terms and definitions
        validation.json     # test/lint commands and conventions
        risks.json          # risky areas and invariants
      doctrine/
        rules.json          # declared coding standards and constraints
      provenance/
        provenance.json     # per-fact lineage records
      guide/
        agent-guide.md      # rendered agent guide
    billing-service/
      ...
```

The binary is `gromit-next` (lives at `cmd/gromit-next/`). It becomes `gromit` when the legacy system is deprecated.

---

## Workspace Resolution

Resolution order:

1. `GROMIT_HOME` environment variable (if set)
2. `$XDG_DATA_HOME/gromit` (if `XDG_DATA_HOME` is set)
3. `~/.local/share/gromit` (default)

The workspace root is created on first use. No configuration file is required.

### Interface

```go
type Resolver interface {
    Resolve() (Root, error)
}

type Root string

func (r Root) ProjectsDir() string
func (r Root) ProjectCell(name string) string
```

---

## Project Attach

`gromit-next project attach /path/to/repo --name payments-api`

Creates a project cell in the workspace. Writes `project.json` containing:

```json
{
  "name": "payments-api",
  "repo_path": "/absolute/path/to/repo",
  "created_at": "2026-03-10T12:00:00Z"
}
```

No files are written to the target repo. The repo path is validated (must exist, must be a git repo). The project name must be unique within the workspace.

### Interface

```go
type Store interface {
    Create(name string, repoPath string) (Cell, error)
    Get(name string) (Cell, error)
    List() ([]Cell, error)
    Delete(name string) error
}

type Cell struct {
    Name       string
    RepoPath   string
    CreatedAt  time.Time
    CellPath   string  // absolute path to project cell directory
}
```

---

## Inspection

`gromit-next project inspect payments-api`

Single command, two internal phases.

### Phase 1: Deterministic Extraction (observed)

Extracts facts from the repo without LLM involvement:

- **Source map:** file inventory with language, line count, last modified
- **Validation commands:** parsed from Makefile, CI config, go.mod, package.json
- **Module structure:** directory tree, package boundaries, import graph
- **Build configuration:** language, framework, dependency manifests

Sources: file system, `go.mod`, `Makefile`, `.github/workflows/`, `package.json`, `Dockerfile`, etc. Each extractor is a pluggable implementation behind an interface.

### Phase 2: LLM Inference (inferred)

Feeds Phase 1 output to an LLM to produce:

- **Architecture summary:** component roles, boundaries, data flow
- **Glossary:** domain terms and definitions
- **Risk areas:** fragile code, complex coupling, areas lacking tests
- **Invariants:** rules the codebase implicitly enforces

The LLM prompt includes Phase 1 artifacts as structured input. The output is parsed into the same artifact format.

### Interfaces

```go
type Inspector interface {
    Inspect(ctx context.Context, cell Cell) (Result, error)
}

type Extractor interface {
    Name() string
    Extract(repoPath string) ([]Fact, error)
}

type Inferrer interface {
    Infer(ctx context.Context, observed []Fact) ([]Fact, error)
}
```

The `Inspector` exposes a single `Inspect` method. The two-phase extract-then-infer flow is an internal implementation detail — callers only care about the combined result. Extractors are registered and run in sequence. Each returns typed facts tagged as `observed`. The `Inferrer` receives observed facts and produces inferred facts.

---

## Knowledge Categories

Every stored fact carries a source category:

| Category | Source | Authority | Example |
|----------|--------|-----------|---------|
| **Declared** | Human author | Highest | "We use hexagonal architecture" |
| **Observed** | Repo extraction | High | "go.mod declares Go 1.22" |
| **Inferred** | LLM analysis | Lowest | "pkg/auth appears to handle JWT validation" |

Declared facts override inferred facts on conflict. Observed facts are ground truth for what they measure.

---

## Doctrine

Doctrine is the declared knowledge layer. Humans author it directly.

```json
{
  "rules": [
    {
      "id": "arch-001",
      "summary": "Use hexagonal architecture with ports and adapters",
      "scope": "architecture",
      "source": "declared",
      "created_at": "2026-03-10T12:00:00Z"
    }
  ]
}
```

Doctrine is loaded from the project cell. It is never auto-generated or auto-modified. The inspection phase reads doctrine to inform its inferences but does not write to it.

### Interface

```go
type Store interface {
    Load(doctrineDir string) (Doctrine, error)
    Save(doctrineDir string, d Doctrine) error
}

type Doctrine struct {
    Rules []Rule
}

type Rule struct {
    ID        string
    Summary   string
    Scope     string
    Source     string    // always "declared"
    CreatedAt time.Time
}
```

---

## Provenance

Every fact carries provenance metadata:

```json
{
  "fact_id": "arch-observed-001",
  "artifact": "architecture.json",
  "category": "observed",
  "git_sha": "abc123",
  "timestamp": "2026-03-10T12:00:00Z",
  "extractor": "go-module-extractor",
  "input_hash": "sha256:..."
}
```

For this spec, freshness is checked at the artifact level: has the repo HEAD changed since the artifact was last generated? Per-fact staleness detection is deferred to a future spec.

### Interface

```go
type Tracker interface {
    Record(rec Record) error
    Check(artifactName string) (Record, error)
    IsFresh(artifactName string, currentSHA string) (bool, error)
}
```

---

## Agent Guide

`gromit-next project guide payments-api`

Renders `agent-guide.md` from project memory. The guide is agent-friendly first: structured sections with consistent headings that any LLM agent can parse reliably. Human-readable as a side effect.

### Sections

1. **Project Overview** — name, language, framework, purpose
2. **Architecture** — component map, boundaries, data flow
3. **Source Map** — key directories and their roles
4. **Validation** — how to test, lint, and verify correctness
5. **Risky Areas** — fragile code, areas requiring extra care
6. **Invariants** — rules the codebase enforces
7. **Glossary** — domain terms and definitions
8. **Doctrine** — declared standards and conventions

Each section is rendered from the corresponding artifact. Sections with no data are omitted rather than rendered empty.

### Interface

```go
type Renderer interface {
    Render(input RenderInput) ([]byte, error)
}

type RenderInput struct {
    ProjectName  string
    Architecture Architecture
    SourceMap    SourceMap
    Validation   Validation
    Risks        []Risk
    Invariants   []Invariant
    Glossary     []GlossaryEntry
    Doctrine     Doctrine
}

type Risk struct {
    Area        string
    Description string
    Severity    string
}

type Invariant struct {
    Rule        string
    Description string
    Scope       string
}
```

---

## Context Compilation

`gromit-next context build payments-api --level project|spec|task`

Compiles minimal, high-signal context packets from project memory. Each level selects relevant facts independently — packets are hierarchical but not cumulative.

### Levels

**Project packet** — broad stable context for project-level work (architecture, doctrine, glossary, validation commands). Used when the agent needs the big picture.

**Spec packet** — spec-relevant slices of project context plus the approved spec text. Includes only the architecture boundaries, risks, and invariants relevant to the spec's scope.

**Task packet** — task-relevant slices of project and spec context plus local proof requirements (which tests to run, which invariants to check, which files are in scope).

### Token Budgeting

Relevance defines scope first. Token budgeting trims within that scope. The budget is a constraint applied after selection, not the selection mechanism.

```go
type Compiler interface {
    Compile(ctx context.Context, cell Cell, level Level, opts CompileOpts) (Packet, error)
}

type CompileOpts struct {
    SpecPath    string
    TaskID      string
    TokenBudget int
}

type Packet struct {
    Level       Level
    Sections    []Section
    TokenCount  int
}

type Section struct {
    Name          string
    Content       string
    TokenEstimate int
    Facts         []FactRef     // provenance references
}

type FactRef struct {
    FactID   string
    Category string
}
```

---

## Storage

All artifacts are JSON files in the project cell. Each artifact type has its own file.

The `Store` interface abstracts storage so the backing implementation can change (e.g., to SQLite) without affecting consumers.

```go
type Store interface {
    Read(cellPath string, artifact string, dest any) error
    Write(cellPath string, artifact string, src any) error
    Exists(cellPath string, artifact string) bool
}
```

`artifact` is a string key like `"architecture"`, `"glossary"`, etc. The store resolves it to the appropriate file path.

---

## Package Structure

```
cmd/
  gromit-next/
    main.go
    project.go       # attach, inspect, guide subcommands
    context.go       # context build subcommand

internal/next/
    workspace/       # workspace resolution
    projectcell/     # project cell CRUD
    inspect/         # inspection orchestration
    extract/         # deterministic extractors
    infer/           # LLM inference phase
    doctrine/        # declared knowledge management
    guide/           # agent guide rendering
    contextpkt/      # context packet compilation
    provenance/      # fact lineage tracking
    artifact/        # artifact storage abstraction
    sourcemap/       # source map utilities
    architecture/    # architecture reasoning
    validation/      # validation result types
    fact/            # fact types and categories
```

Each package exposes interfaces. Concrete implementations are internal to the package.

---

## Design Principles

1. **Interfaces and contracts** — every cross-package boundary is an interface. Concrete implementations are swappable details.
2. **Knowledge categories are explicit** — every fact is tagged declared, observed, or inferred. No unmarked knowledge.
3. **Relevance before budgeting** — context compilation selects by relevance first, trims by token budget second.
4. **No repo writes** — Gromit never writes to the target repository as part of project cell management.
5. **Isolation by default** — project cells share no state. Cross-project learning is a future concern.
6. **Deterministic before probabilistic** — observed facts are extracted deterministically. Inferred facts are clearly labeled and lower authority.

---

## What This Spec Does Not Include

- Autonomous code execution or the execution loop
- Provider/model routing
- Backlog management
- Cross-project learning
- Automatic doctrine rewriting
- Writes to the target repo (AGENTS.md, README, etc.)
