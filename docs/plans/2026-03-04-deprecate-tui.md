# Deprecate TUI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the TUI interface entirely to reduce complexity and dependency weight.

**Architecture:** Delete `internal/tui/` package, `cmd/gromit/tui.go` command, update boundary test and main.go registration, then `go mod tidy` to drop charmbracelet dependencies.

**Tech Stack:** Go, cobra CLI

---

### Task 1: Delete internal/tui/ directory

**Files:**
- Delete: `internal/tui/` (entire directory, 29 files)

**Step 1: Delete the directory**

```bash
rm -rf internal/tui/
```

**Step 2: Verify deletion**

```bash
ls internal/tui/ 2>&1
```
Expected: "No such file or directory"

---

### Task 2: Delete TUI command files

**Files:**
- Delete: `cmd/gromit/tui.go`
- Delete: `cmd/gromit/tui_test.go`

**Step 1: Delete the files**

```bash
rm cmd/gromit/tui.go cmd/gromit/tui_test.go
```

---

### Task 3: Remove TUI command registration from main.go

**Files:**
- Modify: `cmd/gromit/main.go:175`

**Step 1: Remove `tuiCmd` from registerRootCommands**

In `cmd/gromit/main.go`, line 175, delete:
```go
	root.AddCommand(tuiCmd)
```

The function should become:
```go
func registerRootCommands(root *cobra.Command) {
	root.AddCommand(runCmd)
	root.AddCommand(statusCmd)
	root.AddCommand(retroCmd)
	root.AddCommand(validatePRMetadataCmd)
	registerBenchmarkCommands(root)
}
```

**Step 2: Also remove the duplicate registration**

`cmd/gromit/tui.go` had an `init()` with `rootCmd.AddCommand(tuiCmd)` — but since we deleted that file in Task 2, this is already handled. Just confirm no other file registers `tuiCmd`.

```bash
grep -r "tuiCmd" cmd/gromit/
```
Expected: no output (all references deleted)

---

### Task 4: Remove `internal/tui` from delegation boundary test

**Files:**
- Modify: `cmd/gromit/delegation_boundary_test.go:45`

**Step 1: Remove the allowlist entry**

In `cmd/gromit/delegation_boundary_test.go`, delete line 45:
```go
	"github.com/danabrams/gromit/internal/tui":              {},
```

---

### Task 5: Run go mod tidy to drop charmbracelet dependencies

**Step 1: Tidy modules**

```bash
go mod tidy
```

**Step 2: Verify charmbracelet deps are gone**

```bash
grep charmbracelet go.mod
```
Expected: no output (all charmbracelet dependencies removed)

---

### Task 6: Build and test

**Step 1: Verify build**

```bash
go build ./...
```
Expected: clean build, no errors

**Step 2: Run tests**

```bash
go test ./...
```
Expected: all tests pass

---

### Task 7: Commit

**Step 1: Stage and commit**

```bash
git add -A
git commit -m "Remove TUI interface to reduce complexity

Delete internal/tui/ package, cmd/gromit/tui.go command entry point,
and all associated tests. Drop charmbracelet/bubbletea and lipgloss
dependencies via go mod tidy.

Motivation: TUI added maintenance burden and dependency weight
without sufficient value."
```
