#!/usr/bin/env bash
# Test suite for lane-aware timing functionality in test_timing.sh

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

test_count=0
pass_count=0
fail_count=0

# Test helper
assert_lane_detection() {
  local test_name="$1"
  local package="$2"
  local test_file="$3"
  local expected_lane="$4"

  test_count=$((test_count + 1))

  # Detect lane based on test file naming
  local detected_lane="default"
  if [[ "$test_file" =~ smoke ]]; then
    detected_lane="smoke"
  fi

  if [[ "$detected_lane" == "$expected_lane" ]]; then
    echo -e "${GREEN}✓${NC} $test_name"
    pass_count=$((pass_count + 1))
  else
    echo -e "${RED}✗${NC} $test_name (expected $expected_lane, got $detected_lane)"
    fail_count=$((fail_count + 1))
  fi
}

# Test: Smoke tests are detected correctly
assert_lane_detection "smoke_coverage_consolidated_test.go belongs to smoke lane" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "smoke_coverage_consolidated_test.go" \
  "smoke"

# Test: Smoke test files with cmd_smoke pattern
assert_lane_detection "cmd_smoke_slimming_test.go belongs to smoke lane" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "cmd_smoke_slimming_test.go" \
  "smoke"

# Test: Regular tests belong to default lane
assert_lane_detection "debug_test.go belongs to default lane" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "debug_test.go" \
  "default"

# Test: Acceptance tests belong to default lane
assert_lane_detection "debug_agent_acceptance_test.go belongs to default lane" \
  "github.com/danabrams/gromit/cmd/gromit" \
  "debug_agent_acceptance_test.go" \
  "default"

echo
echo "Test Results: $pass_count/$test_count passed"

if [[ $fail_count -gt 0 ]]; then
  exit 1
fi
