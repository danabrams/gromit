#!/usr/bin/env bash
set -euo pipefail

# Pipeline resume hook for Gromit orchestrator skill.
# Fires on SessionStart (after /clear) and injects stage-specific skill content.
#
# This script:
# 1. Checks if .gromit/pipeline-state.json exists
# 2. If no — exits silently (no-op)
# 3. If yes — reads stage field, outputs corresponding skill content + context, deletes state file

STATE_FILE=".gromit/pipeline-state.json"

# Exit silently if no state file exists
if [ ! -f "$STATE_FILE" ]; then
  exit 0
fi

# Parse JSON with jq (preferred) or python3 fallback
parse_json() {
  local file="$1"
  local field="$2"

  if command -v jq >/dev/null 2>&1; then
    jq -r "$field" "$file"
  elif command -v python3 >/dev/null 2>&1; then
    python3 -c "
import json, sys
data = json.load(open(sys.argv[1]))
for key in sys.argv[2].lstrip('.').split('.'):
    data = data[key]
print(data)
" "$file" "$field"
  else
    echo "Error: Neither jq nor python3 available for JSON parsing" >&2
    exit 1
  fi
}

# Read the stage from pipeline state
STAGE=$(parse_json "$STATE_FILE" '.stage')

if [ -z "$STAGE" ] || [ "$STAGE" = "null" ]; then
  echo "Error: No stage field in pipeline state file" >&2
  rm -f "$STATE_FILE"
  exit 1
fi

# Get the directory containing this script (where SKILL.md lives)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_FILE="$SCRIPT_DIR/SKILL.md"

if [ ! -f "$SKILL_FILE" ]; then
  # Fallback to .claude/skills/gromit.md if running from installed location
  SKILL_FILE=".claude/skills/gromit.md"
  if [ ! -f "$SKILL_FILE" ]; then
    echo "Error: Cannot find gromit skill file" >&2
    rm -f "$STATE_FILE"
    exit 1
  fi
fi

# Extract skill content based on stage
extract_skill_content() {
  local stage="$1"
  local start_marker
  local end_marker

  case "$stage" in
    refine)
      start_marker="<!-- BEGIN GROMIT-REFINE-SKILL -->"
      end_marker="<!-- END GROMIT-REFINE-SKILL -->"
      ;;
    plan)
      start_marker="<!-- BEGIN GROMIT-PLAN-SKILL -->"
      end_marker="<!-- END GROMIT-PLAN-SKILL -->"
      ;;
    decompose)
      start_marker="<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->"
      end_marker="<!-- END GROMIT-DECOMPOSE-SKILL -->"
      ;;
    *)
      echo "Error: Unknown stage '$stage'" >&2
      return 1
      ;;
  esac

  # Extract content between markers using sed
  sed -n "/$start_marker/,/$end_marker/p" "$SKILL_FILE" | \
    sed '1d;$d'  # Remove first and last line (the markers themselves)
}

# Build context output based on stage
build_context() {
  local stage="$1"

  case "$stage" in
    refine)
      local idea_text=$(parse_json "$STATE_FILE" '.inputs.idea_text')
      local backlog_id=$(parse_json "$STATE_FILE" '.inputs.backlog_id')

      cat <<'EOF'

---

## Context for This Session

You are refining the following backlog item:

EOF
      printf '**ID:** %s\n' "$backlog_id"
      printf '**Idea:** %s\n' "$idea_text"
      cat <<'EOF'

Please follow the refine methodology to transform this into a structured spec at `.gromit/specs/`.
EOF
      ;;
    plan)
      local spec_name=$(parse_json "$STATE_FILE" '.inputs.spec_name')
      local spec_path=$(parse_json "$STATE_FILE" '.inputs.spec_path')
      local spec_content=$(parse_json "$STATE_FILE" '.inputs.spec_content')
      local open_beads=$(parse_json "$STATE_FILE" '.inputs.open_beads')

      if [ "$open_beads" = "null" ] || [ -z "$open_beads" ]; then
        open_beads="No open beads"
      fi

      cat <<'EOF'

---

## Context for This Session

You are planning implementation for the following spec:

EOF
      printf '**Spec:** %s\n' "$spec_name"
      printf '**Path:** %s\n' "$spec_path"
      printf '\n**Spec Content:**\n%s\n' "$spec_content"
      printf '\n**Open Beads in Project:**\n%s\n' "$open_beads"
      printf '\nPlease follow the plan methodology to create an implementation plan at `.gromit/plans/%s.md`.\n' "$spec_name"
      ;;
    *)
      echo ""
      ;;
  esac
}

# Output skill content and context
extract_skill_content "$STAGE"
build_context "$STAGE"

# Delete state file (consume on read)
rm -f "$STATE_FILE"

exit 0
