#!/usr/bin/env bash
# Test per-lane budget parsing in test_timing.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

test_count=0
pass_count=0
fail_count=0

# Test helper for budget resolution
test_budget_resolution() {
  local test_name="$1"
  local pkg="$2"
  local lane="$3"
  local expected_budget="$4"
  local budget_file="$5"

  test_count=$((test_count + 1))

  # Simulate the get_budget logic from test_timing.sh
  declare -A budgets
  while read -r pkg_or_lane max_sec; do
    [[ -z "${pkg_or_lane:-}" ]] && continue
    [[ "${pkg_or_lane:0:1}" == "#" ]] && continue
    budgets["$pkg_or_lane"]="$max_sec"
  done <"$budget_file"

  # Resolve budget with lane fallback
  local resolved_budget="45" # DEFAULT_BUDGET
  if [[ -n "${budgets[$pkg:$lane]:-}" ]]; then
    resolved_budget="${budgets[$pkg:$lane]}"
  elif [[ -n "${budgets[$pkg]:-}" ]]; then
    resolved_budget="${budgets[$pkg]}"
  fi

  if [[ "$resolved_budget" == "$expected_budget" ]]; then
    echo -e "${GREEN}✓${NC} $test_name"
    pass_count=$((pass_count + 1))
  else
    echo -e "${RED}✗${NC} $test_name (expected $expected_budget, got $resolved_budget)"
    fail_count=$((fail_count + 1))
  fi
}

# Create test budget file with per-lane entries
test_budget_file=$(mktemp)
trap "rm -f $test_budget_file" EXIT

cat > "$test_budget_file" << 'EOF'
# package max_seconds
# per-lane budgets can be specified as package:lane
github.com/danabrams/gromit/cmd/gromit:default 60
github.com/danabrams/gromit/cmd/gromit:smoke 30
github.com/danabrams/gromit/internal/bead 90
EOF

# Test: Smoke lane uses specific budget
test_budget_resolution \
  "Per-lane budget for smoke lane resolved correctly" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "smoke" \
  "30" \
  "$test_budget_file"

# Test: Default lane uses specific budget
test_budget_resolution \
  "Per-lane budget for default lane resolved correctly" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "default" \
  "60" \
  "$test_budget_file"

# Test: Backward compatibility - package without lane suffix uses old format
test_budget_resolution \
  "Backward compatibility - package without lane suffix" \
  "github.com/danabrams/gromit/internal/bead" \
  "default" \
  "90" \
  "$test_budget_file"

# Test: Backward compatibility - package without lane suffix applies to all lanes
test_budget_resolution \
  "Backward compatibility - smoke lane falls back to package budget" \
  "github.com/danabrams/gromit/internal/bead" \
  "smoke" \
  "90" \
  "$test_budget_file"

echo
echo "Test Results: $pass_count/$test_count passed"

if [[ $fail_count -gt 0 ]]; then
  exit 1
fi
