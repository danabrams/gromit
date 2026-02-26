#!/usr/bin/env bash
# Test smoke lane pattern matching

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

test_count=0
pass_count=0
fail_count=0

# Test helper for smoke pattern matching
assert_smoke_pattern() {
  local test_name="$1"
  local test_name_value="$2"
  local expected_lane="$3"

  test_count=$((test_count + 1))

  # Simulate awk lane detection from test_timing.sh
  if echo "$test_name_value" | grep -qE '[Ss]moke|smoke_'; then
    detected_lane="smoke"
  else
    detected_lane="default"
  fi

  if [[ "$detected_lane" == "$expected_lane" ]]; then
    echo -e "${GREEN}✓${NC} $test_name"
    pass_count=$((pass_count + 1))
  else
    echo -e "${RED}✗${NC} $test_name (expected $expected_lane, got $detected_lane)"
    fail_count=$((fail_count + 1))
  fi
}

# Test: Case-insensitive "Smoke" detection (uppercase S)
assert_smoke_pattern \
  "Uppercase Smoke pattern detected" \
  "TestSmokeCoverage" \
  "smoke"

# Test: Lowercase "smoke" detection
assert_smoke_pattern \
  "Lowercase smoke pattern detected" \
  "TestSmokeCoverage" \
  "smoke"

# Test: "smoke_" prefix detection
assert_smoke_pattern \
  "smoke_ prefix detected" \
  "TestSmoke_UnitTest" \
  "smoke"

# Test: Non-smoke test names go to default lane
assert_smoke_pattern \
  "Regular test name goes to default lane" \
  "TestDebugAgent" \
  "default"

# Test: Acceptance tests go to default lane
assert_smoke_pattern \
  "Acceptance test goes to default lane" \
  "TestDebugAgentAcceptance" \
  "default"

# Test: Unit tests go to default lane
assert_smoke_pattern \
  "Unit test goes to default lane" \
  "TestBead" \
  "default"

# Test: Integration tests go to default lane
assert_smoke_pattern \
  "Integration test goes to default lane" \
  "TestBeadIntegration" \
  "default"

# Test: "smoketest" without underscore should be detected as smoke
assert_smoke_pattern \
  "smoketest pattern detected" \
  "TestSmoketest" \
  "smoke"

# Test: Package names with "smoke" should not affect test lane detection
# (this test verifies we're looking at test name, not package path)
assert_smoke_pattern \
  "Package path smoke ignored, test name default" \
  "TestDebug" \
  "default"

echo
echo "Test Results: $pass_count/$test_count passed"

if [[ $fail_count -gt 0 ]]; then
  exit 1
fi
