# Deprecate TUI Interface

**Date:** 2026-03-04
**Status:** Approved
**Motivation:** Reduce complexity — the TUI adds maintenance burden and dependency weight without enough value.

## Approach

Full deletion of all TUI code in a single change. No deprecation period.

## What Gets Deleted

- `internal/tui/` — entire package (~15 files: model, store, views, actions, tabs, keymap, hydration, hints, and tests)
- `cmd/gromit/tui.go` — command entry point
- `cmd/gromit/tui_test.go` — command tests

## What Gets Updated

- `cmd/gromit/delegation_boundary_test.go` — remove `internal/tui` from allowed imports
- `cmd/gromit/main.go` — remove TUI command registration if explicit
- `go.mod` / `go.sum` — `go mod tidy` to drop `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, and transitive deps

## What Stays

- `internal/conversation/` — used by `claude`, `pipeline`, and other core packages

## Verification

- `go build ./...`
- `go test ./...`
