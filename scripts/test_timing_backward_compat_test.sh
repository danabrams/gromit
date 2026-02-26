#!/usr/bin/env bash
# Test backward compatibility with old budget file format

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

test_count=0
pass_count=0
fail_count=0

# Helper to test budget parsing with old format
test_old_budget_format() {
  local test_name="$1"
  local pkg="$2"
  local expected_budget="$3"
  local budget_file="$4"

  test_count=$((test_count + 1))

  # Simulate old budget parsing logic
  declare -A budgets
  while read -r pkg_name max_sec; do
    [[ -z "${pkg_name:-}" ]] && continue
    [[ "${pkg_name:0:1}" == "#" ]] && continue
    budgets["$pkg_name"]="$max_sec"
  done <"$budget_file"

  local resolved_budget="${budgets[$pkg]:-45}"

  if [[ "$resolved_budget" == "$expected_budget" ]]; then
    echo -e "${GREEN}✓${NC} $test_name"
    pass_count=$((pass_count + 1))
  else
    echo -e "${RED}✗${NC} $test_name (expected $expected_budget, got $resolved_budget)"
    fail_count=$((fail_count + 1))
  fi
}

# Create old format budget file (no lane suffixes)
old_budget_file=$(mktemp)
trap "rm -f $old_budget_file" EXIT

cat > "$old_budget_file" << 'EOF'
# package max_seconds
github.com/danabrams/gromit/cmd/gromit 60
github.com/danabrams/gromit/internal/bead 90
github.com/danabrams/gromit/internal/retro 120
github.com/danabrams/gromit/internal/runner 60
EOF

# Test: Old format with package-only entries
test_old_budget_format \
  "Old format: cmd/gromit budget parsed correctly" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "60" \
  "$old_budget_file"

test_old_budget_format \
  "Old format: internal/bead budget parsed correctly" \
  "github.com/danabrams/gromit/internal/bead" \
  "90" \
  "$old_budget_file"

test_old_budget_format \
  "Old format: internal/retro budget parsed correctly" \
  "github.com/danabrams/gromit/internal/retro" \
  "120" \
  "$old_budget_file"

test_old_budget_format \
  "Old format: internal/runner budget parsed correctly" \
  "github.com/danabrams/gromit/internal/runner" \
  "60" \
  "$old_budget_file"

# Test: Default budget fallback for unknown packages
test_old_budget_format \
  "Old format: unknown package uses default budget" \
  "github.com/unknown/package" \
  "45" \
  "$old_budget_file"

echo
echo "Test Results: $pass_count/$test_count passed"

if [[ $fail_count -gt 0 ]]; then
  exit 1
fi
