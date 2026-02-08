---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T23:46:53-05:00"
id: add-context-truncation-fix
source_spec: add-context-truncation-fix
---

# Fix `gromit add` Context Field Truncation — Implementation Plan

**Goal:** Fix `gromit add` so multi-word context input is captured in full, not truncated at the first whitespace.

**Architecture:** Replace `fmt.Scanln(&context)` with `bufio.Scanner` on `os.Stdin` to read the entire line. One-file fix in `cmd/gromit/add.go`.

**Tech Stack:** Go standard library (`bufio`, `os`)

**Spec:** `.gromit/specs/add-context-truncation-fix.md`

---

## Architecture

**Overview:**
The bug is a single `fmt.Scanln` call that stops reading at whitespace. Replace it with `bufio.NewScanner(os.Stdin)` which reads the full line. No changes needed to the backlog storage layer — it already handles multi-word Context strings correctly.

**Key Components:**
1. **`cmd/gromit/add.go`**: The only file with a code change — swap line 80's `fmt.Scanln(&context)` for a `bufio.Scanner`

**Integration Points:**
- The `internal/backlog/` package needs no changes — `Idea.Context` already stores multi-word strings
- The category choice `fmt.Scanln(&choice)` at line 61 stays as-is (reads a single character, whitespace truncation is harmless)
- Backlog JSONL storage/retrieval works correctly for multi-word strings (verified by existing `TestIdeaFieldsRoundtrip`)

**Files to Modify:**
- `cmd/gromit/add.go` — Replace `fmt.Scanln(&context)` with `bufio.Scanner`, add `"bufio"` import

**Tradeoffs:**
- **`bufio.Scanner` over `bufio.NewReader`**: Scanner is idiomatic Go for line-at-a-time input with cleaner error handling
- **CLI integration test over unit test**: The bug is in stdin reading behavior, so an end-to-end test through the binary is more accurate than refactoring `runAdd` to accept an `io.Reader`

## Test Strategy

**Test Levels:**
1. **CLI Integration Test**: End-to-end test using the compiled binary with piped stdin, verifying backlog.jsonl content

**Key Test Cases:**
- Multi-word context captured in full (auto-categorized idea, single stdin line for context)
- Empty context (pressing Enter) produces empty context field
- Unknown category + multi-word context (two stdin lines: category choice + context)

**Test Organization:**
- New `TestCLIContract_AddContextCapture` function in `cmd/gromit/cli_contract_test.go`
- Each subtest creates an isolated temp directory with minimal `gromit.yaml` and sets `cmd.Dir` to prevent writing to the production backlog

**Critical Safety Constraint:**
- All tests MUST run in isolated temp directories with `cmd.Dir` set to the temp dir, ensuring the production `.gromit/backlog.jsonl` is never touched

## Implementation Tasks

### Task 1: Fix context input to read full line

**Files:**
- Modify: `cmd/gromit/add.go`

**What to Do:**
Replace `fmt.Scanln(&context)` on line 80 with a `bufio.Scanner` reading from `os.Stdin`. Add `"bufio"` to imports. Use `scanner.Scan()` + `scanner.Text()` to read the full line. Keep the existing `strings.TrimSpace` call on the result. Leave the category choice `fmt.Scanln(&choice)` at line 61 unchanged.

**Acceptance Criteria:**
- Multi-word context input (e.g. "this should work with the new auth system") is captured in full
- Empty input (pressing Enter with no text) still results in an empty context field
- The category choice prompt (1/2/3) continues to work correctly

**Dependencies:** None

### Task 2: Add CLI integration test for multi-word context capture

**Files:**
- Modify: `cmd/gromit/cli_contract_test.go`

**What to Do:**
Add `TestCLIContract_AddContextCapture` with table-driven subtests. Each subtest creates an isolated temp directory with a minimal `gromit.yaml`, runs the gromit binary with `cmd.Dir` set to the temp dir, pipes stdin input, then reads back `.gromit/backlog.jsonl` from the temp dir to verify the `context` field.

Subtests:
1. **auto-categorized idea with multi-word context**: Idea `"Fix the login bug"` (auto-categorizes as "bug"), stdin `"this should work with the new auth system\n"`, verify context = `"this should work with the new auth system"`
2. **empty context**: Idea `"Fix the login bug"`, stdin `"\n"`, verify context = `""`
3. **unknown category + multi-word context**: Idea `"Think about dashboards"` (triggers category prompt), stdin `"1\nthis is multi-word context\n"`, verify context = `"this is multi-word context"` and type = `"feature"`

Each subtest pattern:
- Create temp dir with `os.MkdirTemp`
- Write minimal `gromit.yaml` to temp dir
- Run `exec.Command(binaryPath, "add", ideaText)` with `cmd.Dir = tmpDir` and `cmd.Stdin = strings.NewReader(stdinInput)`
- Read `filepath.Join(tmpDir, ".gromit", "backlog.jsonl")`
- Parse JSON line and assert on `context` field

**Acceptance Criteria:**
- All tests run in isolated temp directories (production backlog never touched)
- Multi-word context is verified by reading backlog.jsonl from the temp dir
- Empty context edge case is verified
- Category prompt + context prompt interaction is verified

**Dependencies:** Task 1 (the fix must be in place for the test to pass)

---

## Notes

- The `cmd/gromit/cli_contract_test.go` file already has `TestMain` that builds the binary once, so the new test will use the same `binaryPath` variable.
- Import `encoding/json` in the test file if not already present (needed to parse backlog.jsonl).
- The temp dir approach mirrors `TestCLIContract_DecomposePickerBehavior` which already uses this pattern successfully.
