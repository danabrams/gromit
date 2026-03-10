# Gromit Next — Design Document

## Why This Subtree Exists

The existing Gromit implementation (`internal/runner/`, `internal/config/`, etc.) is tightly coupled to repo-local `.gromit/` state and `gromit.yaml` configuration. This works well for single-project use but cannot support:

- **Multi-project workspaces** — managing several repos from one place
- **Workspace-scoped artifacts** — storing derived data outside the target repo
- **Incremental inspection** — reusing analysis across runs without repo pollution
- **Context compilation** — assembling LLM context from structured artifacts rather than raw templates

The `internal/next/` subtree introduces a new architectural center that is **workspace-first and multi-project**, with clear artifact boundaries and provenance tracking.

## How It Differs from Legacy

| Aspect | Legacy (`internal/runner/`, etc.) | Next (`internal/next/`) |
|--------|-----------------------------------|------------------------|
| State location | Repo-local `.gromit/` | User-scoped workspace directory |
| Configuration | `gromit.yaml` in repo root | `project.yaml` per project cell |
| Artifacts | Flat files in `.gromit/` | Typed JSON artifacts with provenance |
| Multi-project | Not supported | First-class concept |
| Context assembly | Template-based prompts | Structured context packets |
| Inspection | Implicit (part of run loop) | Explicit separate phase |

Legacy Gromit continues to work as-is. The new subtree does not import legacy orchestration packages.

## Package Map

```
internal/next/
├── workspace/      — workspace root resolution and initialization
├── projectcell/    — per-project storage cells
├── inspect/        — repo inspection (architecture, source map, glossary)
├── doctrine/       — coding standards and conventions
├── architecture/   — architectural reasoning on inspection output
├── sourcemap/      — source map utilities and file selection
├── validation/     — validation result tracking
├── guide/          — agent-guide.md rendering
├── context/        — context packet compilation
└── provenance/     — artifact lineage and freshness tracking
```

## New CLI Commands

```
gromit project attach [repo-path]   — register a repo as a project
gromit project inspect              — analyze repo, produce artifacts
gromit project guide                — render agent-guide.md
gromit context build                — compile a context packet
```

All commands currently return placeholder messages.

## Intended Migration Direction

1. **Phase 1 (this scaffold):** Establish package boundaries, placeholder types, command skeletons.
2. **Phase 2:** Implement workspace resolution, project registration, and cell storage.
3. **Phase 3:** Implement repo inspection and artifact generation.
4. **Phase 4:** Implement guide rendering and context compilation.
5. **Phase 5:** Wire new context pipeline into the execution loop, replacing legacy template prompts.
6. **Phase 6:** Deprecate `.gromit/` and `gromit.yaml` for projects that have migrated.

Legacy commands (`gromit run`, `gromit add`, etc.) will continue to work throughout migration. The two systems can coexist.

## Out of Scope for This First Slice

The following are explicitly **not implemented** in this scaffold:

- Product behavior (no real inspection, guide rendering, or context compilation)
- Storage backends (no file I/O beyond what's needed to compile)
- Project attachment workflow (no directory creation or YAML writing)
- Repo inspection logic (no AST parsing, file walking, or dependency analysis)
- Guide rendering (no markdown generation)
- Context compilation (no token counting or section assembly)
- Serialization beyond struct tags (no read/write functions)
- Provider/model routing
- Execution loops
- Behavioral tests

## Next Implementation Spec

The first real implementation spec should cover **workspace resolution and project registration** (Phase 2):

- Resolve workspace root from environment/defaults
- Create workspace directory structure on first use
- Implement `gromit project attach` to create a project cell
- Implement `gromit project list` (not yet scaffolded) to enumerate cells
- Write project.yaml to the cell directory
- Add integration tests for the attach/list workflow

## Next Steps (TODO Tracking)

- [ ] **Workspace root resolution** — implement `workspace.Resolver` with XDG/env/fallback strategy
- [ ] **Project registration** — implement `projectcell.Store.Create` with YAML persistence
- [ ] **Project cell storage** — implement full cell directory layout and file management
- [ ] **Repo inspection** — implement `inspect.Inspector` with architecture/source-map/glossary passes
- [ ] **Guide generation** — implement `guide.Renderer` with section rendering and token budgets
- [ ] **Context compilation** — implement `context.Builder` with section selection and budget allocation
- [ ] **Provenance capture** — implement `provenance.Tracker` with git SHA freshness checking
- [ ] **Multi-project isolation tests** — verify cells do not share mutable state
