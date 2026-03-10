# Gromit Next — Agent Guide

## What This Is

Gromit Next is the replacement for the legacy Gromit CLI. It is a workspace-level project memory system that attaches to source repositories without modifying them, maintains isolated project cells outside each repo, inspects projects to build structured context, and compiles minimal context packets for agent consumption.

The binary is `gromit-next`. It lives at `cmd/gromit-next/`. It will replace the legacy `gromit` binary when migration completes.

## Commands

```
gromit-next project attach /path/to/repo --name <name>
gromit-next project inspect <name>
gromit-next project guide <name>
gromit-next context build <name> --level project|spec|task [--spec <path>] [--task <id>]
```

## Workspace Layout

The workspace root resolves in order: `GROMIT_HOME` env var, then `$XDG_DATA_HOME/gromit`, then `~/.local/share/gromit`.

```
$GROMIT_HOME/
  projects/
    <project-name>/
      project.json            # name, repo path, created timestamp
      artifacts/
        architecture.json     # observed + inferred architecture
        sourcemap.json        # file inventory (language, lines, modified)
        glossary.json         # domain terms
        validation.json       # test/lint commands
        risks.json            # risky areas and invariants
      doctrine/
        rules.json            # human-declared coding standards
      provenance/
        provenance.json       # per-fact lineage records
      guide/
        agent-guide.md        # rendered agent guide for this project
```

The agent guide contains the following sections:

1. **Project Overview** — name, language, framework, purpose
2. **Architecture** — component map, boundaries, data flow
3. **Source Map** — key directories and their roles
4. **Validation** — how to test, lint, and verify correctness
5. **Risky Areas** — fragile code, areas requiring extra care
6. **Invariants** — rules the codebase enforces
7. **Glossary** — domain terms and definitions
8. **Doctrine** — declared standards and conventions

## Package Map

All packages live under `internal/next/`. Each exposes interfaces at its boundary.

| Package | Role | Key Interface |
|---------|------|---------------|
| `workspace` | Resolve workspace root | `Resolver` |
| `projectcell` | Project cell CRUD | `Store` |
| `fact` | Fact types and knowledge categories | (types only) |
| `artifact` | JSON artifact storage abstraction | `Store` |
| `extract` | Deterministic repo extractors | `Extractor` |
| `infer` | LLM inference phase | `Inferrer` |
| `inspect` | Inspection orchestrator (extract then infer) | `Inspector` |
| `doctrine` | Declared knowledge management | `Store` |
| `provenance` | Fact lineage and freshness | `Tracker` |
| `guide` | Agent guide renderer | `Renderer` |
| `contextpkt` | Context packet compiler | `Compiler` |
| `sourcemap` | Source map utilities | (helpers) |
| `architecture` | Architecture reasoning utilities | (helpers) |
| `validation` | Validation result types | (types only) |

### Key Interface Signatures

```go
// workspace.Resolver
type Resolver interface {
    Resolve() (Root, error)
}

// projectcell.Store
type Store interface {
    Create(name string, repoPath string) (Cell, error)
    Get(name string) (Cell, error)
    List() ([]Cell, error)
    Delete(name string) error
}

// artifact.Store
type Store interface {
    Read(cellPath string, artifact string, dest any) error
    Write(cellPath string, artifact string, src any) error
    Exists(cellPath string, artifact string) bool
}

// extract.Extractor
type Extractor interface {
    Name() string
    Extract(repoPath string) ([]fact.Fact, error)
}

// inspect.Inspector
type Inspector interface {
    Inspect(ctx context.Context, cell projectcell.Cell) (Result, error)
}

// infer.Inferrer
type Inferrer interface {
    Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error)
}

// doctrine.Store
type Store interface {
    Load(doctrineDir string) (Doctrine, error)
    Save(doctrineDir string, d Doctrine) error
}

// provenance.Tracker
type Tracker interface {
    Record(rec Record) error
    Check(artifactName string) (Record, error)
    IsFresh(artifactName string, currentSHA string) (bool, error)
}

// guide.Renderer
type Renderer interface {
    Render(input RenderInput) ([]byte, error)
}

// contextpkt.Compiler
type Compiler interface {
    Compile(ctx context.Context, cell Cell, level Level, opts CompileOpts) (Packet, error)
}
```

## Scaffold Status

The following packages exist as stubs with placeholder types that MUST be replaced:

- **workspace/** — `Root` is a struct, must become `type Root string`; uses `GROMIT_WORKSPACE` env var, must use `GROMIT_HOME`
- **projectcell/** — uses YAML and `ProjectConfig` struct, must use JSON and simpler `Cell` struct; missing `Delete` method
- **inspect/** — single `Inspect` method, `types.go` has inline domain types that move to their own packages
- **doctrine/** — missing `Source` and `CreatedAt` fields on `Rule`
- **provenance/** — different field names and `Check` returns 3 values instead of 2
- **guide/** — `Render` returns `string` not `[]byte`, `Input` uses flat strings not typed structs
- **contextpkt/** — uses `Builder` not `Compiler`, different method signature, package renamed to `contextpkt`
- **validation/** — models pass/fail results not validation commands
- **sourcemap/** — skeleton only
- **architecture/** — skeleton only

The following packages do not yet exist and must be created from scratch:

- **fact/** — Fact types and knowledge categories
- **extract/** — Deterministic repo extractors
- **infer/** — LLM inference phase
- **artifact/** — JSON artifact storage abstraction
- **cmd/gromit-next/** — CLI binary entry point

## Knowledge Categories

Every stored fact is tagged with one of three categories:

- **declared** — human-authored doctrine. Highest authority. Never auto-generated.
- **observed** — deterministically extracted from the repo (file tree, manifests, CI config). Ground truth for what it measures.
- **inferred** — LLM-generated from observed facts. Lowest authority. Clearly labeled.

Declared overrides inferred on conflict. Observed is ground truth for its domain.

## Inspection Flow

`project inspect` runs two phases in sequence:

1. **Extract** — deterministic extractors run against the repo. Each extractor implements the `Extractor` interface and produces `observed` facts. Sources: file system, go.mod, Makefile, CI config, package.json, Dockerfile.
2. **Infer** — extracted facts are fed to an LLM which produces `inferred` facts: architecture summary, glossary, risk areas, invariants.

Both phases write artifacts to the project cell. Provenance is recorded for every artifact.

## Context Compilation

Context packets are hierarchical but NOT cumulative. Each level selects relevant facts independently from project memory.

- **Project** — broad stable context (architecture, doctrine, glossary, validation)
- **Spec** — spec-relevant slices of project context + the spec text
- **Task** — task-relevant slices of project and spec context + local proof requirements

Relevance selects scope first. Token budgeting trims within that scope second.

## Design Rules

1. Every cross-package boundary is an interface. Concrete implementations are swappable.
2. Every fact carries a source category. No unmarked knowledge.
3. Gromit never writes to the target repository.
4. Project cells share no state. No cross-project leakage.
5. Deterministic extraction runs before probabilistic inference.
6. Storage is JSON files behind an `artifact.Store` interface (SQLite is a future drop-in).

## Tech Stack

- Go (match the version in go.mod)
- Cobra for CLI (match the legacy gromit CLI pattern)
- JSON for artifact storage
- Standard library for file I/O, no ORM

## Testing

- Unit tests per package, colocated (`_test.go` in same package)
- Integration tests use `t.TempDir()` for workspace roots — never touch the real workspace
- Extractors are tested against fixture repos (small directory trees in `testdata/`)
- The LLM inference phase is tested via interface mocks — tests never call a real LLM
- Run tests: `go test ./internal/next/...`
- Run lints: `go vet ./internal/next/...`

## What This Does NOT Include

- Autonomous code execution or the loop
- Provider/model routing
- Backlog management
- Cross-project learning
- Automatic doctrine rewriting
- Writes to the target repository (AGENTS.md, README, etc.)
