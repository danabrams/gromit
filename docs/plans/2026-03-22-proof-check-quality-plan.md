# Proof Check Quality Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a "runtime over source-grep" rule to the planner prompts, and emit a diagnostic when tasks fail on pattern-matching checks while all build checks pass.

**Architecture:** Two surgical edits. (1) Append rule 7 to `buildPlanPrompt` and `buildFixPlanPrompt` in `planner.go`. (2) After all retries are exhausted in `taskloop.go`, classify failing checks and annotate with `[suspect-proof-check]` when only grep/awk checks are failing but builds pass.

**Tech Stack:** Go, standard library only.

**Design doc:** `docs/plans/2026-03-22-proof-check-quality-design.md`

---

### Task 1: Add rule 7 to buildPlanPrompt

**Files:**
- Modify: `internal/next/planner/planner.go:319-320`
- Test: `internal/next/planner/planner_test.go`

**Step 1: Write the failing test**

In `internal/next/planner/planner_test.go`, find the test that checks `buildPlanPrompt` output (search for `TestBuildPlanPrompt` or similar). Add a new assertion:

```go
func TestBuildPlanPrompt_ContainsRuntimeOverSourceGrepRule(t *testing.T) {
    req := PlanRequest{SpecPacket: "spec", Cycle: 1}
    prompt := buildPlanPrompt(req)
    if !strings.Contains(prompt, "Runtime over source-grep") {
        t.Error("expected proof check rule 7 about runtime over source-grep")
    }
    if !strings.Contains(prompt, "--help") {
        t.Error("expected rule 7 to include --help example")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/next/planner/ -run TestBuildPlanPrompt_ContainsRuntimeOverSourceGrepRule -v
```
Expected: FAIL — strings not found.

**Step 3: Add rule 7 to buildPlanPrompt**

In `internal/next/planner/planner.go`, replace the final `return b.String()` in `buildPlanPrompt` (currently right after the rule 6 line at ~319) with:

```go
b.WriteString("\n7. **Runtime over source-grep for behavioral properties**: When verifying that a CLI flag, subcommand, API endpoint, or other user-visible behavior exists, check the *running artifact*, not the source code. Source patterns vary by language and framework (`\"title\"` in Go/cobra, `@click.option('--title')` in Python, `.option('--title')` in JS/commander). The built binary is canonical. Example: `./binary subcommand --help | grep -q -- '--flag-name'`. Use source grep only for implementation structure (call sites, ordering) where no runtime check is possible.\n")
return b.String()
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/next/planner/ -run TestBuildPlanPrompt_ContainsRuntimeOverSourceGrepRule -v
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/next/planner/planner.go internal/next/planner/planner_test.go
git commit -m "feat: add rule 7 (runtime over source-grep) to buildPlanPrompt"
```

---

### Task 2: Add rule 7 to buildFixPlanPrompt

**Files:**
- Modify: `internal/next/planner/planner.go` (buildFixPlanPrompt, around line 250-258)
- Test: `internal/next/planner/planner_test.go`

The fix-plan prompt already has inline proof check rules at lines 253-258. Append the same rule 7 after the existing rule about `*_test.go` files.

**Step 1: Write the failing test**

```go
func TestBuildFixPlanPrompt_ContainsRuntimeOverSourceGrepRule(t *testing.T) {
    req := FixPlanRequest{Cycle: 2, Failures: []string{"some failure"}}
    prompt := buildFixPlanPrompt(req)
    if !strings.Contains(prompt, "Runtime over source-grep") {
        t.Error("expected proof check rule about runtime over source-grep in fix plan prompt")
    }
    if !strings.Contains(prompt, "--help") {
        t.Error("expected rule to include --help example in fix plan prompt")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/next/planner/ -run TestBuildFixPlanPrompt_ContainsRuntimeOverSourceGrepRule -v
```
Expected: FAIL.

**Step 3: Add rule 7 after the existing `*_test.go` rule**

In `buildFixPlanPrompt`, find the line:
```go
b.WriteString("    - For `*_test.go` in `expected_touched_area`, include a proof check verifying new test content exists. Do NOT rely solely on `go test ./...`.\n")
```

Append immediately after:
```go
b.WriteString("    - **Runtime over source-grep for behavioral properties**: For CLI flags, subcommands, or user-visible behaviors, check the built binary: `./binary subcommand --help | grep -q -- '--flag-name'`. Source patterns vary by language/framework. Use source grep only for call sites and ordering where no runtime check is possible.\n")
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/next/planner/ -run TestBuildFixPlanPrompt_ContainsRuntimeOverSourceGrepRule -v
```
Expected: PASS.

**Step 5: Run full planner tests**

```bash
go test ./internal/next/planner/ -v
```
Expected: all pass.

**Step 6: Commit**

```bash
git add internal/next/planner/planner.go internal/next/planner/planner_test.go
git commit -m "feat: add runtime-over-source-grep rule to buildFixPlanPrompt"
```

---

### Task 3: Add isBuildCheck helper to taskloop

**Files:**
- Modify: `internal/next/specloop/taskloop.go`
- Test: `internal/next/specloop/shell_task_inspector_test.go` (or a new `taskloop_classify_test.go`)

This helper classifies a proof check command as a build/compile check vs. a pattern-matching check.

**Step 1: Write the failing test**

Create `internal/next/specloop/taskloop_classify_test.go`:

```go
package specloop

import "testing"

func TestIsBuildCheck(t *testing.T) {
    cases := []struct {
        cmd  string
        want bool
    }{
        {"go build ./...", true},
        {"go build ./internal/next/planner/...", true},
        {"go vet ./...", true},
        {"npm run build", true},
        {"cargo build", true},
        {"mvn compile", true},
        {"make build", true},
        {"grep -q 'func Reject' internal/next/proposaltriage/promote.go", false},
        {"grep -q '--title' cmd/gromit-next/review_proposals.go", false},
        {"awk '/stepA/{ a=NR }' file.go", false},
        {"go test -run TestFoo ./...", false},
        {"./binary --help | grep -q -- '--flag'", false},
    }
    for _, c := range cases {
        got := isBuildCheck(c.cmd)
        if got != c.want {
            t.Errorf("isBuildCheck(%q) = %v, want %v", c.cmd, got, c.want)
        }
    }
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/next/specloop/ -run TestIsBuildCheck -v
```
Expected: FAIL — `isBuildCheck` undefined.

**Step 3: Implement isBuildCheck**

Add to `internal/next/specloop/taskloop.go` (near the top, after imports):

```go
// isBuildCheck returns true if cmd is a build or compilation check (go build,
// go vet, npm run build, cargo build, mvn compile, make build). These are
// treated as harder evidence than pattern-matching checks (grep, awk, sed).
func isBuildCheck(cmd string) bool {
    cmd = strings.TrimSpace(cmd)
    buildPrefixes := []string{
        "go build ",
        "go build\t",
        "go vet ",
        "go vet\t",
        "npm run build",
        "cargo build",
        "mvn compile",
        "make build",
    }
    // Handle "go build ./..." with no trailing space
    if cmd == "go build ./..." || cmd == "go vet ./..." {
        return true
    }
    for _, prefix := range buildPrefixes {
        if strings.HasPrefix(cmd, prefix) {
            return true
        }
    }
    return false
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/next/specloop/ -run TestIsBuildCheck -v
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/next/specloop/taskloop.go internal/next/specloop/taskloop_classify_test.go
git commit -m "feat: add isBuildCheck helper for proof check classification"
```

---

### Task 4: Add suspect-proof-check diagnostic in taskloop

**Files:**
- Modify: `internal/next/specloop/taskloop.go` (around line 310)
- Test: `internal/next/specloop/taskloop_classify_test.go`

**Step 1: Write the failing test**

Add to `taskloop_classify_test.go`:

```go
func TestAnnotateSuspectProofChecks_AllBuildPass(t *testing.T) {
    proofChecks := []string{
        "go build ./...",
        "grep -q '--title' cmd/gromit-next/review_proposals.go",
        "grep -q '--change' cmd/gromit-next/review_proposals.go",
    }
    failures := []string{
        "grep -q '--title' cmd/gromit-next/review_proposals.go: exit status 1",
        "grep -q '--change' cmd/gromit-next/review_proposals.go: exit status 1",
    }
    result := annotateSuspectProofChecks(proofChecks, failures)
    for _, f := range result {
        if !strings.HasPrefix(f, "[suspect-proof-check]") {
            t.Errorf("expected suspect prefix on %q", f)
        }
    }
}

func TestAnnotateSuspectProofChecks_BuildFailing(t *testing.T) {
    proofChecks := []string{
        "go build ./...",
        "grep -q 'func Foo' internal/foo.go",
    }
    failures := []string{
        "go build ./...: exit status 1: undefined: Bar",
        "grep -q 'func Foo' internal/foo.go: exit status 1",
    }
    result := annotateSuspectProofChecks(proofChecks, failures)
    for _, f := range result {
        if strings.HasPrefix(f, "[suspect-proof-check]") {
            t.Errorf("should NOT have suspect prefix when build is also failing: %q", f)
        }
    }
}

func TestAnnotateSuspectProofChecks_NoProofChecks(t *testing.T) {
    result := annotateSuspectProofChecks(nil, []string{"some failure"})
    if strings.HasPrefix(result[0], "[suspect-proof-check]") {
        t.Error("should not annotate when no proof checks to classify")
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/next/specloop/ -run TestAnnotateSuspectProofChecks -v
```
Expected: FAIL — `annotateSuspectProofChecks` undefined.

**Step 3: Implement annotateSuspectProofChecks**

Add to `taskloop.go`:

```go
// annotateSuspectProofChecks annotates failure messages with [suspect-proof-check]
// when all build/compile checks in proofChecks are passing but only pattern-matching
// checks (grep, awk, etc.) are failing. This signals to the fix planner that the
// implementation may be correct and the proof checks need to be rewritten to be
// more behavioral rather than re-implementing already-correct code.
//
// If any build check appears in failures (i.e. is also failing), no annotation is
// added — the failure is likely a genuine implementation problem.
// If proofChecks is empty, failures are returned unchanged.
func annotateSuspectProofChecks(proofChecks []string, failures []string) []string {
    if len(proofChecks) == 0 || len(failures) == 0 {
        return failures
    }

    // Check if any build check is also failing
    for _, f := range failures {
        for _, pc := range proofChecks {
            if isBuildCheck(pc) && strings.Contains(f, pc) {
                // A build check is failing — not a suspect proof check situation
                return failures
            }
        }
    }

    // Check that at least one build check exists in the task's proof checks
    // (i.e. we have evidence the build passes, not just absence of build failures)
    hasBuildCheck := false
    for _, pc := range proofChecks {
        if isBuildCheck(pc) {
            hasBuildCheck = true
            break
        }
    }
    if !hasBuildCheck {
        return failures
    }

    // All build checks pass, only pattern-matching checks are failing
    annotated := make([]string, len(failures))
    for i, f := range failures {
        annotated[i] = "[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. " + f
    }
    return annotated
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/next/specloop/ -run TestAnnotateSuspectProofChecks -v
```
Expected: PASS.

**Step 5: Wire into taskloop — call annotateSuspectProofChecks before marking failed**

In `taskloop.go`, find the block (around line 309-312):
```go
// If inspection still fails after all retries, mark as failed
if !ir.Pass {
    result.Status = "failed"
}
```

Replace with:
```go
// If inspection still fails after all retries, mark as failed.
// Annotate failures with [suspect-proof-check] if only pattern-matching
// checks are failing while build checks all pass — signals to the fix
// planner that proof checks may need rewriting, not the implementation.
if !ir.Pass {
    ir.Failures = annotateSuspectProofChecks(entry.task.ProofChecks, ir.Failures)
    result.Status = "failed"
}
```

**Note:** `ir.Failures` flows into the retry/fix context so the fix planner will see the annotation. Verify this by checking where `ir.Failures` is consumed after this block.

**Step 6: Run full specloop tests**

```bash
go test ./internal/next/specloop/ -v
```
Expected: all pass.

**Step 7: Commit**

```bash
git add internal/next/specloop/taskloop.go internal/next/specloop/taskloop_classify_test.go
git commit -m "feat: annotate suspect proof check failures when build passes"
```

---

### Task 5: Run full test suite and verify

**Step 1: Run all affected packages**

```bash
go test ./internal/next/planner/... ./internal/next/specloop/... -v 2>&1 | tail -20
```
Expected: all pass.

**Step 2: Build the binary**

```bash
go build ./...
```
Expected: success.

**Step 3: Commit if any cleanup needed, otherwise done**

If all green, the implementation is complete.
