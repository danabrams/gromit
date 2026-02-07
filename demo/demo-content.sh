#!/usr/bin/env bash
# demo-content.sh — Pre-seeded project content for gromit demo
# Source this file; do not execute directly.
# All heredocs use single-quoted delimiters to prevent variable expansion.

# write_starter_project <dir>
# Writes a minimal Go todo-cli project that builds, tests, and vets cleanly.
write_starter_project() {
  local dir="$1"
  if [[ -z "$dir" ]]; then
    echo "Usage: write_starter_project <dir>"
    return 1
  fi

  mkdir -p "$dir/todo"

  cat > "$dir/go.mod" << 'EOF'
module todo-cli

go 1.23
EOF

  cat > "$dir/main.go" << 'EOF'
package main

import (
	"fmt"
	"os"
	"strconv"

	"todo-cli/todo"
)

func main() {
	store := todo.NewStore("todos.json")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: todo-cli add <title>")
			os.Exit(1)
		}
		title := os.Args[2]
		t, err := store.Add(title)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added: [%d] %s\n", t.ID, t.Title)

	case "list":
		todos, err := store.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(todos) == 0 {
			fmt.Println("No todos yet. Add one with: todo-cli add \"Buy milk\"")
			return
		}
		for _, t := range todos {
			status := " "
			if t.Done {
				status = "x"
			}
			fmt.Printf("[%s] %d: %s\n", status, t.ID, t.Title)
		}

	case "done":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: todo-cli done <id>")
			os.Exit(1)
		}
		id, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid ID: %s\n", os.Args[2])
			os.Exit(1)
		}
		if err := store.MarkDone(id); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Marked todo %d as done\n", id)

	case "help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: todo-cli <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  add <title>   Add a new todo")
	fmt.Println("  list          List all todos")
	fmt.Println("  done <id>     Mark a todo as done")
	fmt.Println("  help          Show this help")
}
EOF

  cat > "$dir/todo/todo.go" << 'EOF'
package todo

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

// Todo represents a single todo item.
type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

// Store manages todo persistence using a JSON file.
type Store struct {
	path string
}

// NewStore creates a Store that reads and writes to the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Add creates a new todo with the given title and saves it.
func (s *Store) Add(title string) (Todo, error) {
	todos, err := s.load()
	if err != nil {
		return Todo{}, err
	}

	nextID := 1
	for _, t := range todos {
		if t.ID >= nextID {
			nextID = t.ID + 1
		}
	}

	t := Todo{
		ID:        nextID,
		Title:     title,
		Done:      false,
		CreatedAt: time.Now(),
	}
	todos = append(todos, t)

	if err := s.save(todos); err != nil {
		return Todo{}, err
	}
	return t, nil
}

// List returns all todos from the store.
func (s *Store) List() ([]Todo, error) {
	return s.load()
}

// MarkDone marks the todo with the given ID as done.
func (s *Store) MarkDone(id int) error {
	todos, err := s.load()
	if err != nil {
		return err
	}

	for i := range todos {
		if todos[i].ID == id {
			todos[i].Done = true
			return s.save(todos)
		}
	}

	return errors.New("todo not found")
}

func (s *Store) load() ([]Todo, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var todos []Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

func (s *Store) save(todos []Todo) error {
	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
EOF

  cat > "$dir/todo/todo_test.go" << 'EOF'
package todo

import (
	"path/filepath"
	"testing"
)

func TestAdd(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "todos.json"))

	todo, err := store.Add("Buy milk")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if todo.ID != 1 {
		t.Errorf("expected ID 1, got %d", todo.ID)
	}
	if todo.Title != "Buy milk" {
		t.Errorf("expected title 'Buy milk', got %q", todo.Title)
	}
	if todo.Done {
		t.Error("expected Done to be false")
	}
}

func TestList(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "todos.json"))

	store.Add("First")
	store.Add("Second")

	todos, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}
	if todos[0].Title != "First" {
		t.Errorf("expected first todo 'First', got %q", todos[0].Title)
	}
	if todos[1].Title != "Second" {
		t.Errorf("expected second todo 'Second', got %q", todos[1].Title)
	}
}

func TestMarkDone(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "todos.json"))

	todo, _ := store.Add("Test task")
	err := store.MarkDone(todo.ID)
	if err != nil {
		t.Fatalf("MarkDone failed: %v", err)
	}

	todos, _ := store.List()
	if !todos[0].Done {
		t.Error("expected todo to be marked done")
	}
}

func TestMarkDoneNotFound(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "todos.json"))

	err := store.MarkDone(999)
	if err == nil {
		t.Error("expected error for non-existent todo")
	}
}
EOF

  cat > "$dir/CLAUDE.md" << 'EOF'
# Todo CLI

A simple command-line todo manager written in Go.

## Build & Test

```bash
go build ./...    # Build
go test ./...     # Test
go vet ./...      # Vet
```

## Project Structure

```
todo-cli/
├── main.go          # CLI entry point with os.Args routing
├── todo/
│   ├── todo.go      # Todo struct and Store (JSON file persistence)
│   └── todo_test.go # Unit tests
├── go.mod           # Module definition (no external dependencies)
└── CLAUDE.md        # This file
```

## Key Patterns

- **No external dependencies** — stdlib only, keeps the project simple
- **File-based storage** — todos saved as JSON array in `todos.json`
- **Subcommand routing** — `os.Args[1]` switch in main, no framework
- **Test isolation** — each test uses `t.TempDir()` for its own store file
- **Sequential IDs** — auto-incremented, max existing ID + 1
EOF

  cat > "$dir/.gitignore" << 'EOF'
# Binary
todo-cli

# Data
todos.json

# IDE
.idea/
.vscode/
*.swp
*~
EOF

  echo "Starter project written to $dir"
}

# write_fallback_spec <dir>
# Writes a pre-built spec for due-dates feature (used with --skip mode).
write_fallback_spec() {
  local dir="$1"
  if [[ -z "$dir" ]]; then
    echo "Usage: write_fallback_spec <dir>"
    return 1
  fi

  mkdir -p "$dir/.gromit/specs"

  cat > "$dir/.gromit/specs/due-dates.md" << 'EOF'
---
id: due-dates
source_ideas:
  - "Add due dates with natural language parsing"
created: 2026-01-15
---

# Due Dates

## Specification

Add due date support to the todo CLI. Users can set due dates when creating todos using natural language (e.g., "tomorrow", "next friday", "3 days") or ISO 8601 format (e.g., "2026-03-15"). Todos display their due dates and can be sorted by urgency.

## Acceptance Criteria

1. `todo-cli add "Buy milk" --due tomorrow` creates a todo with a due date set to tomorrow at midnight UTC
2. `todo-cli add "File taxes" --due 2026-04-15` creates a todo with the specified ISO date
3. Relative date expressions supported: "today", "tomorrow", "N days", "next monday" through "next sunday"
4. `todo-cli list` displays due dates as "due: Jan 15" next to each todo that has one
5. `todo-cli list --sort due` sorts todos by due date (soonest first, no-date items last)
6. Invalid date expressions produce a clear error message and do not create a todo
7. All date parsing logic has unit tests with deterministic time injection

## Decisions

- **Separate `dateparse` package** — keeps parsing logic isolated and testable without touching the store
- **`DueDate *time.Time` field** — pointer allows omitempty for backward compatibility with existing todos.json
- **`--due` flag on add command** — uses `flag.NewFlagSet` since main.go uses os.Args routing
- **`--sort due` flag on list** — new flag rather than always sorting by due date
- **Midnight UTC normalization** — all relative dates resolve to midnight UTC of the target day

## Research & Context

- `todo/todo.go` — Todo struct will need a new DueDate field
- `main.go` — add command needs --due flag, list command needs --sort flag
- `todo/todo_test.go` — existing test patterns use t.TempDir() for isolation
- No external date parsing library — stdlib `time.Parse` for ISO, custom logic for relative dates
EOF

  echo "Fallback spec written to $dir/.gromit/specs/due-dates.md"
}

# write_fallback_plan <dir>
# Writes a pre-built plan for due-dates feature (used with --skip mode).
write_fallback_plan() {
  local dir="$1"
  if [[ -z "$dir" ]]; then
    echo "Usage: write_fallback_plan <dir>"
    return 1
  fi

  mkdir -p "$dir/.gromit/plans"

  cat > "$dir/.gromit/plans/due-dates.md" << 'EOF'
---
spec: due-dates
created: 2026-01-15
---

# Due Dates — Implementation Plan

## Overview

Add due date support to the todo CLI in five incremental steps. Each task is self-contained and produces working, tested code.

## Tasks

### 1. Create dateparse package

**Files:** `dateparse/dateparse.go`, `dateparse/dateparse_test.go`
**Dependencies:** none

Create a `dateparse` package with a single exported function:

```go
func ParseFrom(input string, now time.Time) (time.Time, error)
```

Supported expressions:
- `today` — returns `now` normalized to midnight UTC
- `tomorrow` — midnight UTC of next day
- `N days` — midnight UTC of now + N days (e.g., "3 days")
- `next monday` through `next sunday` — midnight UTC of next occurrence
- ISO 8601 (`2006-01-02`) — parsed directly with `time.Parse`

Return a clear error for unrecognized input. Inject `now` parameter for deterministic testing.

Tests: one test case per expression type, plus error cases.

### 2. Add DueDate field to Todo struct

**Files:** `todo/todo.go`, `todo/todo_test.go`
**Dependencies:** none

Add `DueDate *time.Time` field to the `Todo` struct with `json:"due_date,omitempty"`. Verify existing tests still pass (backward compatibility with existing todos.json that lack the field). Add a test that round-trips a todo with a due date through JSON.

### 3. Add --due flag to add command

**Files:** `main.go`
**Dependencies:** 1, 2

Parse `--due` flag from `os.Args` using `flag.NewFlagSet("add", flag.ExitOnError)`. When provided, call `dateparse.ParseFrom(dueStr, time.Now())` and set the `DueDate` field. Print the due date in the confirmation message.

### 4. Add due date display and --sort flag to list command

**Files:** `main.go`
**Dependencies:** 2

Display due dates in list output as `(due: Jan 02)` after the title for todos that have a due date. Add `--sort due` flag using `flag.NewFlagSet("list", flag.ExitOnError)`. When sorting, todos with due dates come first (sorted ascending), followed by todos without due dates.

### 5. Integration tests and CLAUDE.md update

**Files:** `todo/todo_test.go`, `CLAUDE.md`
**Dependencies:** 3, 4

Add integration-style tests that exercise the full flow: create todos with due dates, list them, verify sort order. Update CLAUDE.md to document the new `--due` and `--sort` flags and the `dateparse` package.
EOF

  echo "Fallback plan written to $dir/.gromit/plans/due-dates.md"
}

# write_seed_learnings <dir>
# Writes example learnings to LEARNINGS.md for the retro demo.
write_seed_learnings() {
  local dir="$1"
  if [[ -z "$dir" ]]; then
    echo "Usage: write_seed_learnings <dir>"
    return 1
  fi

  cat > "$dir/.gromit/LEARNINGS.md" << 'EOF'
# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

*No confirmed learnings.*

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-01-15 | demo-001 | patterns
Use `t.TempDir()` for test file isolation instead of creating temp files manually. The test framework handles cleanup automatically and each test gets its own directory, preventing state leakage between tests.

### 2026-01-15 | demo-002 | conventions
Use `flag.NewFlagSet` per subcommand when the CLI routes via `os.Args` switch. This avoids polluting the global flag set and allows each subcommand to define its own flags independently.

### 2026-01-15 | demo-003 | gotchas
Use pointer types (`*time.Time`) with `json:"omitempty"` for optional fields added to existing JSON-persisted structs. This ensures backward compatibility — existing files without the field unmarshal correctly as nil.

### 2026-01-15 | demo-004 | patterns
Accept an injected `now` parameter (e.g., `ParseFrom(input string, now time.Time)`) instead of calling `time.Now()` internally. This makes date/time logic deterministic in tests without test-only hooks or global overrides.

---

## Archived

*No longer relevant or superseded.*
EOF

  echo "Seed learnings written to $dir/.gromit/LEARNINGS.md"
}
