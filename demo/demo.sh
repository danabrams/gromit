#!/usr/bin/env bash
# demo.sh — Interactive gromit demo script
# Usage: source demo/demo.sh && demo_setup
#
# Call functions one at a time for a recorded walkthrough:
#   demo_setup      — Create project, git init, bd init, gromit init
#   demo_add        — Add 2 ideas via gromit add
#   demo_backlog    — Show backlog
#   demo_refine     — Refine an idea into a spec (--skip for fallback)
#   demo_plan       — Create implementation plan (--skip for fallback)
#   demo_decompose  — Decompose plan into beads
#   demo_queue      — Show bead queue
#   demo_run        — Run gromit on 2 beads
#   demo_board      — Show board + git log
#   demo_review     — Interactive code review
#   demo_retro      — Retrospective on learnings
#   demo_cleanup    — Remove demo directory

DEMO_DIR="${DEMO_DIR:-/tmp/gromit-demo}"
# Support both bash (BASH_SOURCE) and zsh (%x prompt expansion)
if [[ -n "${BASH_SOURCE[0]}" ]]; then
  DEMO_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
else
  DEMO_SCRIPT_DIR="$(cd "$(dirname "${(%):-%x}")" && pwd)"
fi
source "$DEMO_SCRIPT_DIR/demo-content.sh"

# Resolve gromit binary
if [[ -n "$GROMIT_BIN" ]]; then
  : # use env var as-is
elif [[ -x "$(dirname "$DEMO_SCRIPT_DIR")/gromit" ]]; then
  GROMIT_BIN="$(dirname "$DEMO_SCRIPT_DIR")/gromit"
elif command -v gromit &>/dev/null; then
  GROMIT_BIN="gromit"
else
  echo "Error: cannot find gromit binary. Set GROMIT_BIN or build with 'go build -o gromit ./cmd/gromit'"
  return 1 2>/dev/null || exit 1
fi

# _demo_header <number> <title> <description>
_demo_header() {
  local num="$1" title="$2" desc="$3"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Step $num: $title"
  echo "  $desc"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
}

demo_setup() {
  _demo_header 1 "Project Setup" "Create project, git init, bd init, gromit init"

  if [[ ! -x "$GROMIT_BIN" ]] && ! command -v "$GROMIT_BIN" &>/dev/null; then
    echo "Error: gromit binary not found at $GROMIT_BIN"
    return 1
  fi

  rm -rf "$DEMO_DIR"
  mkdir -p "$DEMO_DIR"
  cd "$DEMO_DIR" || return 1

  echo ">>> Writing starter Go project..."
  write_starter_project "$DEMO_DIR"

  echo ""
  echo ">>> Initializing git..."
  git init -b main
  git config user.email "demo@example.com"
  git config user.name "Demo User"
  git add -A
  git commit -m "Initial todo-cli project"

  echo ""
  echo ">>> Initializing bd..."
  bd init -p demo-todo

  echo ""
  echo ">>> Initializing gromit..."
  "$GROMIT_BIN" init

  echo ""
  echo ">>> Fixing validation commands for Go..."
  sed -i 's|"pnpm run test"|"go test ./..."|' gromit.yaml
  sed -i 's|"pnpm run lint:check"|"go vet ./..."|' gromit.yaml
  sed -i 's|"pnpm run build"|"go build ./..."|' gromit.yaml

  echo ""
  echo ">>> Committing gromit scaffolding..."
  git add -A
  git commit -m "Add gromit scaffolding"

  echo ""
  echo ">>> Project structure:"
  find . -not -path './.git/*' | head -30

  echo ""
  echo "Setup complete! Project is at $DEMO_DIR"
}

demo_add() {
  _demo_header 2 "Add Ideas" "Add 2 feature ideas to the backlog"

  cd "$DEMO_DIR" || return 1

  echo '>>> Adding idea: "Add due dates with natural language parsing"'
  printf "\n" | "$GROMIT_BIN" add "Add due dates with natural language parsing"

  echo ""
  echo '>>> Adding idea: "Improve error messages when todo not found"'
  printf "\n" | "$GROMIT_BIN" add "Improve error messages when todo not found"

  echo ""
  echo "Done! Run demo_backlog to see the ideas."
}

demo_backlog() {
  _demo_header 3 "View Backlog" "Show all ideas in the backlog"

  cd "$DEMO_DIR" || return 1

  "$GROMIT_BIN" backlog
}

demo_refine() {
  _demo_header 4 "Refine Idea → Spec" "Turn an idea into a detailed specification"

  cd "$DEMO_DIR" || return 1

  if [[ "$1" == "--skip" ]]; then
    echo ">>> Skipping interactive refine — writing fallback spec..."
    write_fallback_spec "$DEMO_DIR"
    git add -A
    git commit -m "Add due-dates spec"
  else
    echo ">>> Select the 'due dates' idea when prompted..."
    "$GROMIT_BIN" refine
  fi
}

demo_plan() {
  _demo_header 5 "Plan Implementation" "Create an implementation plan from the spec"

  cd "$DEMO_DIR" || return 1

  if [[ "$1" == "--skip" ]]; then
    echo ">>> Skipping interactive plan — writing fallback plan..."
    write_fallback_plan "$DEMO_DIR"
    git add -A
    git commit -m "Add due-dates plan"
  else
    echo ">>> Select the 'due-dates' spec when prompted..."
    "$GROMIT_BIN" plan
  fi
}

demo_decompose() {
  _demo_header 6 "Decompose Plan → Beads" "Break the plan into atomic work units"

  cd "$DEMO_DIR" || return 1

  "$GROMIT_BIN" decompose due-dates --review
}

demo_queue() {
  _demo_header 7 "View Queue" "Show beads ready to be worked on"

  cd "$DEMO_DIR" || return 1

  "$GROMIT_BIN" queue
}

demo_run() {
  _demo_header 8 "Run Gromit" "Execute the build loop on 2 beads"

  cd "$DEMO_DIR" || return 1

  "$GROMIT_BIN" run -n 2
}

demo_board() {
  _demo_header 9 "View Board" "Show bead status and recent commits"

  cd "$DEMO_DIR" || return 1

  "$GROMIT_BIN" board

  echo ""
  echo ">>> Recent commits:"
  git log --oneline -10
}

demo_review() {
  _demo_header 10 "Code Review" "Review the changes made by gromit"

  cd "$DEMO_DIR" || return 1

  "$GROMIT_BIN" review
}

demo_retro() {
  _demo_header 11 "Retrospective" "Review and improve learnings"

  cd "$DEMO_DIR" || return 1

  # Seed learnings if LEARNINGS.md is empty or has no provisional entries
  if ! grep -q "^### " "$DEMO_DIR/.gromit/LEARNINGS.md" 2>/dev/null; then
    echo ">>> Seeding example learnings for demo..."
    write_seed_learnings "$DEMO_DIR"
  fi

  "$GROMIT_BIN" retro
}

demo_cleanup() {
  _demo_header 12 "Cleanup" "Remove demo directory"

  read -p "Remove $DEMO_DIR? [y/N] " confirm
  if [[ "$confirm" =~ ^[Yy]$ ]]; then
    rm -rf "$DEMO_DIR"
    echo "Removed $DEMO_DIR"
    cd ~ || true
  else
    echo "Skipped cleanup. Demo dir is at $DEMO_DIR"
  fi
}

echo "Demo functions loaded! Start with: demo_setup"
echo "Demo directory: $DEMO_DIR"
echo "Gromit binary: $GROMIT_BIN"
