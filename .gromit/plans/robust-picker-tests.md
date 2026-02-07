---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T05:59:51-05:00"
id: robust-picker-tests
source_spec: robust-picker-tests
---

# Robust Picker Tests Implementation Plan

**Goal:** Replace hardcoded numeric picker indices in tests with a computed helper and consolidate three copies of `runGromitWithStdin` into a shared test package.

**Architecture:** A new `test/testutil` package provides two exported functions — `PickerStdin` (computes picker selection stdin from type/target/count) and `RunGromitWithStdin` (consolidated process runner taking primitive params). Each test package keeps thin local wrappers that delegate to testutil, preserving existing call signatures.

**Tech Stack:** Go, standard library only (os/exec, fmt, strings)

**Spec:** `.gromit/specs/robust-picker-tests.md`

---

## Architecture

### Shared Package: `test/testutil`

**`test/testutil/picker.go`** — Picker index calculator:
```go
func PickerStdin(pickerType, target string, n int) string
```
- `pickerType`: `"refine"`, `"plan"`, or `"decompose"`
- `target`: `"something_new"`, `"decompose_all"`, or `"item"`
- `n`: for `"item"` — 1-based position; for special targets — total item count
- Returns the stdin string (e.g., `"3\n"`)
- Panics on unknown picker type (fail fast in tests)

Picker layout logic encoded:
- Refine: items 1..N, "Something new" at N+1
- Plan: items 1..N, no special options
- Decompose: items 1..N, "Decompose all" at N+1

**`test/testutil/runner.go`** — Consolidated process runner:
```go
func RunGromitWithStdin(binary, dir string, environ []string, stdin string, args ...string) (stdout, stderr string, exitCode int, err error)
```
- Uses pipe-based stdin approach (from e2e/contracts implementations)
- Empty `dir` → don't set `cmd.Dir`; nil `environ` → inherit parent environment
- Returns 4 values: stdout, stderr, exitCode, err
- Non-zero exit codes are not errors (exitCode set, err=nil)
- Only returns err for infrastructure failures (pipe creation, command not found)

### Thin Wrappers (per test package)

Each package keeps its existing function signatures but delegates to testutil:

- **`test/e2e/e2e_test.go`**: `runGromitWithStdin(env *e2eEnv, stdin string, args ...string)` → calls `testutil.RunGromitWithStdin(gromitBinary, env.Dir, env.Env, stdin, args...)`
- **`test/contracts/helpers_test.go`**: `runGromitWithStdin(env *testEnv, stdin string, args ...string)` → calls `testutil.RunGromitWithStdin(gromitBinary, env.Dir, env.Env, stdin, args...)`
- **`cmd/gromit/cli_contract_test.go`**: `runGromitWithStdin(t *testing.T, stdin string, args ...string)` → calls `testutil.RunGromitWithStdin(binaryPath, "", nil, stdin, args...)`, adapts 4-return to 3-return by calling `t.Fatal` on err

The non-stdin variants (`runGromit`, `runGromitWithEnv`) delegate similarly with empty stdin.

## Test Strategy

**Unit tests** (`test/testutil/picker_test.go`):
- Table-driven tests for `PickerStdin` covering all picker types and targets
- Refine: "something_new" with various item counts, "item" at various positions
- Plan: "item" at various positions
- Decompose: "decompose_all" with various item counts, "item" at various positions
- Edge cases: 0 items, 1 item, large counts
- Unknown picker type panics (recover-based test)

**Integration validation**: All existing E2E, contract, and CLI contract tests pass unchanged — pure refactor of test infrastructure.

No separate unit tests for `RunGromitWithStdin` — it's exercised by every existing test through the wrappers.

## Implementation Tasks

### Task 1: Create picker stdin helper

**Files:**
- Create: `test/testutil/picker.go`
- Create: `test/testutil/picker_test.go`

**What to Do:**
Create the `testutil` package with `PickerStdin` function that encodes each picker's layout logic. Implement table-driven unit tests covering all picker types, targets, and edge cases.

`PickerStdin` logic:
- `"refine"` + `"something_new"`: return `fmt.Sprintf("%d\n", n+1)` (n = item count)
- `"refine"` + `"item"`: return `fmt.Sprintf("%d\n", n)` (n = 1-based position)
- `"plan"` + `"item"`: return `fmt.Sprintf("%d\n", n)`
- `"decompose"` + `"decompose_all"`: return `fmt.Sprintf("%d\n", n+1)`
- `"decompose"` + `"item"`: return `fmt.Sprintf("%d\n", n)`
- Unknown pickerType: `panic`

**Acceptance Criteria:**
- `PickerStdin("refine", "something_new", 2)` returns `"3\n"`
- `PickerStdin("plan", "item", 2)` returns `"2\n"`
- All table-driven tests pass

**Dependencies:** None

### Task 2: Create consolidated runner

**Files:**
- Create: `test/testutil/runner.go`

**What to Do:**
Create `RunGromitWithStdin` function using the pipe-based implementation from `test/e2e/e2e_test.go` (lines 296-344). Parameterize by binary path, dir, environ, stdin, and args. Handle empty dir (don't set `cmd.Dir`) and nil environ (don't set `cmd.Env`).

**Acceptance Criteria:**
- Function compiles with correct signature: `(binary, dir string, environ []string, stdin string, args ...string) (stdout, stderr string, exitCode int, err error)`
- Empty dir and nil environ are handled (no crash, inherits defaults)
- Non-zero exit codes return exitCode with nil error

**Dependencies:** None

### Task 3: Update E2E tests to use testutil

**Files:**
- Modify: `test/e2e/e2e_test.go`
- Modify: `test/e2e/refine_e2e_test.go`

**What to Do:**
In `e2e_test.go`: replace the bodies of `runGromit` (lines 264-292) and `runGromitWithStdin` (lines 296-344) with one-line delegations to `testutil.RunGromitWithStdin`. Add `"github.com/danabrams/gromit/test/testutil"` import.

In `refine_e2e_test.go`: replace `stdin := "3\n"` (line 248) with `stdin := testutil.PickerStdin("refine", "something_new", 2)` — the 2 comes from the test's own fixture setup (2 unrefined backlog items). Replace `stdin := "1\n"` (line 437) with `stdin := testutil.PickerStdin("refine", "item", 1)`. Add testutil import.

**Acceptance Criteria:**
- No hardcoded numeric stdin values remain in `refine_e2e_test.go`
- `runGromit` and `runGromitWithStdin` in `e2e_test.go` are thin wrappers (< 5 lines each)
- All E2E tests pass: `go test -tags e2e ./test/e2e/...`

**Dependencies:** Task 1, Task 2

### Task 4: Update contract tests to use testutil

**Files:**
- Modify: `test/contracts/helpers_test.go`

**What to Do:**
Replace the bodies of `runGromitWithEnv` (lines 178-206) and `runGromitWithStdin` (lines 210-258) with delegations to `testutil.RunGromitWithStdin`. `runGromitWithEnv` delegates with empty stdin. Add testutil import.

**Acceptance Criteria:**
- `runGromitWithEnv` and `runGromitWithStdin` are thin wrappers (< 5 lines each)
- All contract tests pass: `go test ./test/contracts/...`

**Dependencies:** Task 2

### Task 5: Update CLI contract tests to use testutil

**Files:**
- Modify: `cmd/gromit/cli_contract_test.go`
- Modify: `cmd/gromit/stdin_helper_example_test.go`

**What to Do:**
In `cli_contract_test.go`: replace the bodies of `runGromit` (lines 52-72) and `runGromitWithStdin` (lines 76-97) with delegations to `testutil.RunGromitWithStdin`. These wrappers need to adapt the 4-return to 3-return by calling `t.Fatalf` when `err != nil`. Pass `""` for dir and `nil` for environ since CLI contract tests don't use a custom environment.

In `stdin_helper_example_test.go`: replace `stdin := "3\n"` (line 14) with `stdin := testutil.PickerStdin("refine", "something_new", 2)` to demonstrate the helper. Add testutil import.

**Acceptance Criteria:**
- `runGromit` and `runGromitWithStdin` are thin wrappers (< 8 lines each, including `t.Fatal` adaptation)
- Example test uses `PickerStdin` instead of hardcoded `"3\n"`
- All CLI contract tests pass: `go test ./cmd/gromit/...`

**Dependencies:** Task 1, Task 2

---

## Notes

- The decompose picker is not yet implemented, but `PickerStdin` already supports it with `"decompose"` type and `"decompose_all"` target. When the decompose picker lands, tests can use the helper immediately.
- The `"item"` target works identically across all picker types — it's always just the 1-based position passed through. The picker type parameter is still required for validation and future-proofing.
- Tasks 1 and 2 have no dependencies and can be done in parallel. Tasks 3, 4, 5 can also be parallelized once their dependencies are met.
