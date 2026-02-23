---
id: gemini-cli-spike
source_spec: gemini-cli-spike
created: 2026-02-23
decomposed: false
---

# Gemini CLI Spike Implementation Plan

**Goal:** Empirically verify the Gemini CLI invocation contract and produce fixture-grade findings that de-risk `GeminiProvider` implementation.

**Architecture:** Execute a deterministic 9-area CLI research matrix, capture raw outputs (JSONL, JSON, stderr, exit codes), and synthesize exact schemas and error patterns into a single findings document with provider-oriented recommendations.

**Tech Stack:** Gemini CLI (`gemini`), shell tooling (`bash`/`zsh`), markdown documentation artifacts

**Spec:** `.gromit/specs/gemini-cli-spike.md`

---

## Architecture

### Overview

This is a research-only spike. No production Go code is added. The deliverable is a reproducible findings package containing raw captures and a structured analysis that answers all nine verification areas required before implementing `GeminiProvider`.

### Key Components

1. **Preflight and environment matrix**
   - Validate CLI availability/version and auth preconditions.
   - Distinguish setup failures from Gemini behavior failures early.

2. **Nine-area execution matrix**
   - Run a fixed set of commands for prompt delivery, streaming schema, JSON schema, token/cost reporting, model validity, exit codes, error patterns, tool permissions, and working directory behavior.

3. **Raw artifact capture**
   - Save complete stdout/stderr/exit-code evidence for each section.
   - Persist at least one complete successful `stream-json` run and one complete successful `json` run as fixture candidates.

4. **Schema and classification synthesis**
   - Derive exact event/object schemas and stable error-string patterns from captures.
   - Map findings to future parser/classifier touchpoints for `GeminiProvider`.

5. **Findings report**
   - Produce `.gromit/plans/gemini-cli-spike-findings.md` with command evidence, observed behavior, implementation implications, and limitations.

### Integration Points

- Unblocks the upcoming `gemini-provider-adapter` implementation work.
- Aligns with existing provider result expectations in `internal/provider/provider.go` (token/cost/failure fields).
- Provides fixture candidates for future provider contract tests in `test/contracts` / provider parsing tests.
- Mirrors the precedent established by `codex-cli-spike` findings.

### Data Flow

1. Preflight checks establish runnable environment.
2. Execute per-section commands and capture raw outputs.
3. Normalize and store captures as fixture candidates/evidence.
4. Extract schemas/error signatures and document invariants/ambiguities.
5. Publish final findings doc with explicit recommendations.

### Files to Modify

- `.gromit/plans/gemini-cli-spike-findings.md` - Final spike findings document

### Files to Create

- `.gromit/plans/fixtures/gemini/stream-json-success.jsonl` - Complete stream-json fixture candidate
- `.gromit/plans/fixtures/gemini/json-success.json` - Complete json fixture candidate
- `.gromit/plans/fixtures/gemini/errors/*.stderr.txt` - Error pattern evidence samples
- `.gromit/plans/fixtures/gemini/commands.log` - Command ledger with timestamps and exit codes

### Tradeoffs

- **Real CLI observation over mocks:** Chosen for contract accuracy; requires a working/authenticated environment.
- **Raw evidence first, synthesis second:** Improves traceability at modest execution overhead.
- **No code changes in spike phase:** Keeps risk low and avoids speculative parser implementation.

## Test Strategy

### Test Levels

1. **CLI Observation Tests**
   - Real command executions for all nine verification areas with full capture of outputs and return codes.

2. **Artifact Validation Checks**
   - Verify captured JSON/JSONL payloads are parseable and complete for fixture candidacy.

3. **Manual Cross-Checks**
   - Reconcile conclusions against raw captures before finalizing schema and error-classification claims.

### Key Test Cases

- Prompt delivery mode coverage: inline `-p`, large prompt via `-p` (50KB+), stdin pipe, and `-p "@file"` inclusion semantics.
- Streaming event schema via `--output-format stream-json`: event types, field names/types/nesting, text accumulation behavior, and result-event ordering.
- Non-streaming structured schema via `--output-format json`: `response`, `stats`, and `error` object shapes.
- Token/cost presence and semantics: input/output tokens, cached tokens, direct cost fields or absence.
- Model selection outcomes: supported model names and invalid-model error payload shape.
- Exit code semantics: explicit evidence for 0, 1, 42, and 53 (or explicit not-observed notes with attempted triggers).
- Error pattern capture for classifier design: auth, rate limit, and transport/disconnect signatures.
- Tool permission behavior in headless mode: available flags, defaults, and non-interactive execution constraints.
- Working directory behavior: inherited CWD, override capabilities, and cross-directory file access behavior.

### Mocking Strategy

- No behavioral mocks for Gemini CLI itself.
- Shell helpers allowed only for input generation/capture organization.
- If a failure class is non-triggerable, document attempted triggers and retain raw evidence for what was observed.

### Coverage Goals

- All nine spec areas answered with command-backed evidence.
- One complete `stream-json` success capture saved.
- One complete `json` success capture saved.
- Real stderr samples captured for each triggerable error class.
- Prompt delivery recommendation selected with explicit rationale tied to observed limits/behavior.

### Test Organization

- Findings: `.gromit/plans/gemini-cli-spike-findings.md`
- Raw captures: `.gromit/plans/fixtures/gemini/`
- Section format in findings: `Commands Run` -> `Observed Output` -> `Implementation Implications`

## Implementation Tasks

### Task 1: Preflight Gemini environment and define capture harness

**Files:**
- Create: `.gromit/plans/fixtures/gemini/commands.log`
- Create: `.gromit/plans/fixtures/gemini/preflight.md`

**What to Do:**
Establish a repeatable preflight procedure (binary/version, auth readiness, baseline command sanity) and a capture harness convention for stdout/stderr/exit code logging used by all subsequent tasks.

**Acceptance Criteria:**
- Preflight records Gemini binary/version and auth readiness state.
- Capture harness conventions are defined and used consistently.
- Every subsequent task can append reproducible command + exit code evidence to `commands.log`.

**Dependencies:** None

**Notes:**
If preflight fails, findings must clearly separate environment blocker from Gemini contract conclusions.

### Task 2: Execute prompt delivery and output-schema investigations

**Files:**
- Create: `.gromit/plans/fixtures/gemini/prompt-delivery/`
- Create: `.gromit/plans/fixtures/gemini/stream-json-success.jsonl`
- Create: `.gromit/plans/fixtures/gemini/json-success.json`
- Create: `.gromit/plans/fixtures/gemini/schema-notes.md`

**What to Do:**
Run and capture the first five investigation areas: prompt delivery modes, stream-json schema/events, json schema/stats, token/cost fields, and model-selection behavior (including invalid model errors).

**Acceptance Criteria:**
- Inline/ststdin/large-prompt/@file behaviors are captured and compared.
- One full successful stream-json transcript is stored as fixture candidate.
- One full successful json output is stored as fixture candidate.
- Model support and invalid-model error format are documented with raw evidence.

**Dependencies:** Task 1

**Notes:**
Keep raw captures unedited; perform normalization only in derived notes.

### Task 3: Execute error, exit-code, permissions, and CWD investigations

**Files:**
- Create: `.gromit/plans/fixtures/gemini/errors/`
- Create: `.gromit/plans/fixtures/gemini/permissions/permissions-notes.md`
- Create: `.gromit/plans/fixtures/gemini/workdir/workdir-notes.md`

**What to Do:**
Run and capture the remaining areas: exit-code triggers (0/1/42/53), auth/rate-limit/transport error patterns, tool permission flags/default behavior, and working-directory semantics.

**Acceptance Criteria:**
- Exit code observations include stderr samples and trigger commands.
- Error-classification section includes real, reusable string patterns for each triggerable category.
- Tool permission controls and defaults are explicitly documented.
- CWD inheritance and override behavior are confirmed with command evidence.

**Dependencies:** Task 1

**Notes:**
For non-triggerable cases (for example, rate-limit or transport), include attempted methods and unresolved risk notes.

### Task 4: Author findings document with provider-oriented recommendations

**Files:**
- Modify: `.gromit/plans/gemini-cli-spike-findings.md`

**What to Do:**
Compile all captures into the final findings document, including exact schemas, recommended prompt delivery mode, token/cost handling recommendations, error classification patterns, permission model conclusions, and working-directory guidance for future `GeminiProvider` implementation.

**Acceptance Criteria:**
- Findings document covers all nine areas with evidence-backed conclusions.
- Includes fixture candidate references and paths.
- Includes explicit recommendation for Gromit prompt delivery mode with rationale.
- Confirms spike scope is research-only and no production code changes were made.

**Dependencies:** Task 2, Task 3

**Notes:**
Use the same style as prior Codex findings for consistency and easier downstream consumption.

---

## Notes

- This plan intentionally produces documentation and fixture artifacts only.
- Any discovered follow-up implementation work should be tracked as new beads linked via `discovered-from`.
- If Gemini CLI behavior diverges by version/auth mode, the findings should include versioned caveats rather than over-generalizing.
