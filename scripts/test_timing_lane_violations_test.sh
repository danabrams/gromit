#!/usr/bin/env bash
# Test that budget violations include lane context

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

test_count=0
pass_count=0
fail_count=0

# Test helper for violation message format
assert_violation_format() {
  local test_name="$1"
  local message="$2"
  local should_contain="$3"

  test_count=$((test_count + 1))

  if echo "$message" | grep -q "$should_contain"; then
    echo -e "${GREEN}✓${NC} $test_name"
    pass_count=$((pass_count + 1))
  else
    echo -e "${RED}✗${NC} $test_name"
    echo "  Message: $message"
    echo "  Should contain: $should_contain"
    fail_count=$((fail_count + 1))
  fi
}

# Test 1: Default lane violation message includes lane context
violation_msg="Budget exceeded [default lane]: github.com/danabrams/gromit/cmd/gromit took 65s (budget 60s)"
assert_violation_format \
  "Default lane violation includes [default lane] context" \
  "$violation_msg" \
  "\[default lane\]"

# Test 2: Smoke lane violation message includes lane context
violation_msg="Budget exceeded [smoke lane]: github.com/danabrams/gromit/cmd/gromit took 35s (budget 30s)"
assert_violation_format \
  "Smoke lane violation includes [smoke lane] context" \
  "$violation_msg" \
  "\[smoke lane\]"

# Test 3: Violation message includes package name
violation_msg="Budget exceeded [default lane]: github.com/danabrams/gromit/cmd/gromit took 65s (budget 60s)"
assert_violation_format \
  "Violation includes package name" \
  "$violation_msg" \
  "github.com/danabrams/gromit/cmd/gromit"

# Test 4: Violation message includes actual and budget times
violation_msg="Budget exceeded [smoke lane]: github.com/danabrams/gromit/cmd/gromit took 35s (budget 30s)"
assert_violation_format \
  "Violation includes actual time and budget" \
  "$violation_msg" \
  "took 35s (budget 30s)"

echo
echo "Test Results: $pass_count/$test_count passed"

if [[ $fail_count -gt 0 ]]; then
  exit 1
fi
