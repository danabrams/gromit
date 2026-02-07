---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T05:43:44-05:00"
id: harden-fake-claude-writes
source_spec: harden-fake-claude-writes
---

# Harden Fake Claude Script File Writes — Implementation Plan

**Goal:** Add path validation and safe content writing to the fake claude script's file write feature.

**Architecture:** Add a validation block (TEST_DIR required, absolute path required, path containment via `realpath -m`) before the existing mkdir/write, replace `echo` with `printf '%s\n'`, and add contract tests for each error path.

**Tech Stack:** Bash (fake script), Go (contract tests)

**Spec:** `.gromit/specs/harden-fake-claude-writes.md`

---

## Architecture

**Overview:**
Insert validation checks into the file write block of `test/fakes/claude` and add error-path contract tests.

**Key Components:**
1. **`test/fakes/claude` validation block**: Three checks (TEST_DIR set, absolute path, path containment) before mkdir/write. Errors exit 1 with descriptive stderr. Replace `echo` with `printf '%s\n'`.
2. **`test/contracts/fakes_integration_test.go` error path tests**: Four new test functions covering each validation failure plus backslash content fidelity.

**Integration Points:**
- Existing happy-path test (`TestFakes_ClaudeWriteFile`) unchanged — uses absolute path inside TEST_DIR
- E2E tests (`refine_e2e_test.go`) all use absolute paths inside TEST_DIR — no changes needed
- Error message pattern matches existing script conventions (e.g., `CLAUDE_FIXTURE` errors)

**Files to Modify:**
- `test/fakes/claude` — Add validation, replace echo with printf
- `test/contracts/fakes_integration_test.go` — Add error-path and backslash-content tests

## Test Strategy

**Contract Tests** (in `test/contracts/fakes_integration_test.go`):
- `TestFakes_ClaudeWriteFile_MissingTestDir`: CLAUDE_WRITE_FILE set, TEST_DIR unset → exit 1, stderr mentions TEST_DIR
- `TestFakes_ClaudeWriteFile_RelativePath`: CLAUDE_WRITE_FILE is relative → exit 1, stderr mentions "absolute path"
- `TestFakes_ClaudeWriteFile_PathTraversal`: CLAUDE_WRITE_FILE resolves outside TEST_DIR → exit 1, stderr mentions "outside TEST_DIR" or "path traversal"
- `TestFakes_ClaudeWriteFile_BackslashContent`: Content with `hello\nworld` written literally, no escape interpretation
- Existing `TestFakes_ClaudeWriteFile`: Regression — happy path still works

**No mocking needed** — direct script invocations with controlled env vars.

## Implementation Tasks

### Task 1: Add validation and printf to fake claude script

**Files:**
- Modify: `test/fakes/claude`

**What to Do:**
Replace the file write block (lines 20-29) with a validated version:
1. When `CLAUDE_WRITE_FILE` is set, check `TEST_DIR` is set — if not, print error to stderr and `exit 1`
2. Check `CLAUDE_WRITE_FILE` starts with `/` — if not, print error to stderr and `exit 1`
3. Resolve both `TEST_DIR` and `CLAUDE_WRITE_FILE` with `realpath -m`, verify the file path starts with the resolved `TEST_DIR` prefix — if not, print error to stderr and `exit 1`
4. Replace `echo "$CLAUDE_WRITE_CONTENT"` with `printf '%s\n' "$CLAUDE_WRITE_CONTENT"`

**Acceptance Criteria:**
- Validation errors exit 1 with descriptive stderr messages
- `printf '%s\n'` used instead of `echo`
- Happy path (valid absolute path inside TEST_DIR) still works

**Dependencies:** None

### Task 2: Add contract tests for validation error paths

**Files:**
- Modify: `test/contracts/fakes_integration_test.go`

**What to Do:**
Add four test functions after the existing `TestFakes_ClaudeWriteFile`:
1. `TestFakes_ClaudeWriteFile_MissingTestDir` — remove `TEST_DIR` from env, set `CLAUDE_WRITE_FILE`, expect exit 1 + stderr mentions `TEST_DIR`
2. `TestFakes_ClaudeWriteFile_RelativePath` — set `CLAUDE_WRITE_FILE` to `relative/path.txt`, expect exit 1 + stderr mentions "absolute path"
3. `TestFakes_ClaudeWriteFile_PathTraversal` — set `CLAUDE_WRITE_FILE` to `TEST_DIR/../../etc/passwd`, expect exit 1 + stderr mentions "outside TEST_DIR" or "path traversal"
4. `TestFakes_ClaudeWriteFile_BackslashContent` — set content to `hello\nworld`, verify file contains literal `hello\nworld\n` (no escape interpretation)

**Acceptance Criteria:**
- All four new tests pass
- Existing `TestFakes_ClaudeWriteFile` still passes
- Each error test verifies both exit code and stderr message content

**Dependencies:** Task 1

---

## Notes

- `realpath -m` is GNU coreutils — available on all Linux targets. Does not require the path to exist.
- The `TestFakes_ClaudeWriteFile_MissingTestDir` test must explicitly remove `TEST_DIR` from the env slice since `setupTestEnv` sets it by default.
- The backslash content test should use Go's raw string literal to set content containing literal `\n` characters (not actual newlines).
