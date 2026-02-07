---
id: robust-picker-tests
source_ideas: []
created: 2026-02-07
---

# Robust Picker Tests

## Specification

E2E and contract tests that interact with CLI pickers (refine, plan, decompose) currently hardcode numeric stdin values to select picker options (e.g., `"3\n"` to select "Something new"). This is fragile — if backlog items are added, reordered, or picker options change, the hardcoded index silently selects the wrong item rather than failing loudly.

Replace all hardcoded picker indices with a shared test helper that computes the correct index from the test's own fixture data. The helper encodes each picker's layout logic (e.g., "Something new" is always `len(items)+1` in the refine picker) and returns the appropriate stdin string for a given target label or item.

Additionally, consolidate the three separate copies of `runGromitWithStdin` (in `test/e2e/e2e_test.go`, `test/contracts/helpers_test.go`, and `cmd/gromit/cli_contract_test.go`) into a single shared test helper package.

### Picker Index Helper

A helper function that takes a picker type and a target selection, and returns the stdin string needed to select it. For example:

- `pickerStdin("refine", "Something new", 2)` → `"3\n"` (2 unrefined items, "Something new" is always last)
- `pickerStdin("refine", "item", 1)` → `"1\n"` (select the 1st backlog item)
- `pickerStdin("plan", "spec", 2)` → `"2\n"` (select the 2nd unplanned spec)

The helper makes test intent explicit — code reads as "select Something new" rather than "send 3".

### Shared Test Helpers Package

A new shared package (e.g., `test/testutil/`) that contains:
- The picker stdin helper
- The consolidated `runGromitWithStdin` function (adapted to work across E2E and contract test contexts)

## Acceptance Criteria

- No hardcoded numeric picker indices remain in E2E or contract tests; all picker selections use the shared helper
- A shared picker stdin helper exists that computes the correct index given picker type, target label, and item count
- `runGromitWithStdin` is consolidated into a single shared implementation used by all test packages
- All existing E2E tests (`refine_e2e_test.go`, `e2e_test.go`) pass using the new helpers
- All existing contract tests pass using the consolidated `runGromitWithStdin`
- The decompose picker (when implemented) can use the same helper pattern

## Decisions

1. **Compute-from-fixture over two-pass parsing.** A two-pass approach (run command to capture picker output, parse it, run again with correct index) was considered but rejected. The picker blocks on stdin, so capturing output before providing input would require pty/goroutine complexity. Since tests control the fixture data, they already know what the picker will show — computing the index from that knowledge is simpler, faster, and sufficient.

2. **Consolidate `runGromitWithStdin` as part of this work.** The three copies share the same purpose and mostly the same implementation. Consolidating them reduces maintenance burden and provides a natural home for the picker helper. The shared package needs to accommodate slightly different test contexts (E2E uses `*e2eEnv`, contracts uses `*testEnv`, CLI contracts use `*testing.T` directly).

3. **One helper covers all picker types.** Rather than separate helpers per picker, a single function parameterized by picker type keeps the API surface small and makes it easy to add new pickers (like decompose) without creating new functions.

## Research & Context

### Current State

**Hardcoded indices in tests:**
- `test/e2e/refine_e2e_test.go:248` — `stdin := "3\n"` selects "Something new" (assumes 2 unrefined items)
- `test/e2e/refine_e2e_test.go:438` — `stdin := "1\n"` selects first backlog item
- `cmd/gromit/stdin_helper_example_test.go:14` — `stdin := "3\n"` example

**Three copies of `runGromitWithStdin`:**
- `test/e2e/e2e_test.go:296-344` — takes `*e2eEnv`, creates stdin pipe, returns stdout/stderr/exitCode/error
- `test/contracts/helpers_test.go:210-258` — takes `*testEnv`, same pipe-based approach
- `cmd/gromit/cli_contract_test.go:76-97` — takes `*testing.T`, simpler `strings.NewReader` approach

**Picker implementations (all hand-rolled, same pattern):**
- `cmd/gromit/refine.go:63-127` — numbered list + "Something new..." as last option, reads number from stdin
- `cmd/gromit/plan.go:70-109` — numbered list of unplanned specs, reads number from stdin
- `cmd/gromit/decompose.go` — decompose picker is planned (spec exists at `.gromit/specs/decompose-picker.md`) but not yet implemented
