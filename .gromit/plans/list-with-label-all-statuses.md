---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T07:08:10-05:00"
id: list-with-label-all-statuses
source_spec: list-with-label-all-statuses
---

# Fix ListWithLabel to Return All Statuses and Unlimited Results — Implementation Plan

**Goal:** Fix `ListWithLabel` to return both open and closed beads without a 50-result cap.

**Architecture:** Add `--all` and `--limit 0` flags to the single `c.run()` call in `ListWithLabel`. No new methods, no interface changes.

**Tech Stack:** Go

**Spec:** `.gromit/specs/list-with-label-all-statuses.md`

---

## Architecture

**Overview:**
Single-line change to `ListWithLabel` in `internal/bead/bead.go` — add `"--all", "--limit", "0"` to the `c.run()` arguments. All four callers benefit automatically.

**Integration Points:**
- `epic.go:282` — `getBeadCounts` will now correctly count closed beads
- `review.go:161,215` — commit detection will find commits from closed beads
- `main.go:298` — filter set will include closed bead IDs

**Files to Modify:**
- `internal/bead/bead.go` — Add `--all` and `--limit 0` to `ListWithLabel` command (line 616)
- `internal/bead/label_integration_test.go` — Update `IntegrationCallsCorrectCommand` (line 256) and `CommandContract` (line 456) expected commands

## Test Strategy

**Unit tests (`list_with_label_test.go`, `bead_test.go`):** No changes needed — these test JSON parsing via helpers, not command args.

**Integration tests (`label_integration_test.go`):** Two tests hardcode the expected `bd` command and must be updated:
1. `TestListWithLabel_IntegrationCallsCorrectCommand` (line 256) — update `exec.Command` args
2. `TestListWithLabel_CommandContract` (line 456) — update `expectedCmd` slice

**Mock tests:** No changes — interface signature is unchanged.

## Implementation Tasks

### Task 1: Add --all and --limit 0 to ListWithLabel and update integration tests

**Files:**
- Modify: `internal/bead/bead.go`
- Modify: `internal/bead/label_integration_test.go`

**What to Do:**
1. In `internal/bead/bead.go` line 616, change:
   ```go
   out, err := c.run("list", "--json", "--label", label)
   ```
   to:
   ```go
   out, err := c.run("list", "--json", "--label", label, "--all", "--limit", "0")
   ```

2. In `internal/bead/label_integration_test.go` line 256 (`IntegrationCallsCorrectCommand`), update:
   ```go
   cmd := exec.Command("bd", "list", "--json", "--label", testLabel)
   ```
   to:
   ```go
   cmd := exec.Command("bd", "list", "--json", "--label", testLabel, "--all", "--limit", "0")
   ```

3. In `internal/bead/label_integration_test.go` line 456 (`CommandContract`), update:
   ```go
   expectedCmd := []string{"bd", "list", "--json", "--label", testLabel}
   ```
   to:
   ```go
   expectedCmd := []string{"bd", "list", "--json", "--label", testLabel, "--all", "--limit", "0"}
   ```

**Acceptance Criteria:**
- `ListWithLabel` passes `--all` and `--limit 0` to the `bd list` command
- `getBeadCounts` correctly reports non-zero closed counts when closed beads exist
- Integration tests reflect the new command arguments

**Dependencies:** None

---

## Notes

- This is a minimal, high-confidence change. The `--all` flag is documented by `bd list --help` and the `--limit 0` pattern is already used by `List()`.
- No caller changes needed — all four call sites benefit from receiving complete data.
