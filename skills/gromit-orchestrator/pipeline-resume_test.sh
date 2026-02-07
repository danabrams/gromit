#!/usr/bin/env bash
# Test suite for pipeline-resume.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$SCRIPT_DIR/pipeline-resume.sh"
STATE_FILE=".gromit/pipeline-state.json"

# Test counter
TESTS_RUN=0
TESTS_PASSED=0

# Test helper
run_test() {
  local test_name="$1"
  local expected="$2"
  local json_content="$3"

  TESTS_RUN=$((TESTS_RUN + 1))

  # Create state file
  echo "$json_content" > "$STATE_FILE"

  # Run script and capture output
  local output
  output=$("$SCRIPT" 2>&1) || true

  # Check if expected string is in output
  if echo "$output" | grep -F "$expected" > /dev/null; then
    echo "✓ PASS: $test_name"
    TESTS_PASSED=$((TESTS_PASSED + 1))
  else
    echo "✗ FAIL: $test_name"
    echo "  Expected to find: $expected"
    echo "  Got output:"
    echo "$output" | sed 's/^/    /'
    return 1
  fi
}

# Setup
mkdir -p .gromit

echo "Testing pipeline-resume.sh dollar-sign handling..."
echo

# Test 1: Dollar sign in spec content
run_test "Dollar signs in spec_content" \
  '$VARIABLE and ${ANOTHER}' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Code with $VARIABLE and ${ANOTHER} variables",
    "open_beads": "None"
  }
}'

# Test 2: Double dollar signs ($$)
run_test "Double dollar signs" \
  '$$' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Shell PID is $$",
    "open_beads": "None"
  }
}'

# Test 3: Dollar signs in open_beads
run_test "Dollar signs in open_beads" \
  'Fix $PATH' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Test spec",
    "open_beads": "Bead 1: Fix $PATH expansion"
  }
}'

# Test 4: Complex Go template syntax
run_test "Go template syntax" \
  '{{ .Foo }}' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Template: {{ .Foo }} with $VAR",
    "open_beads": "None"
  }
}'

# Test 5: Backticks and command substitution syntax
run_test "Command substitution syntax" \
  '$(command)' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Example: $(command) and `backtick`",
    "open_beads": "None"
  }
}'

# Test 6: Refine stage with dollar signs
run_test "Refine stage with dollar signs" \
  'Implement $feature' \
  '{
  "stage": "refine",
  "inputs": {
    "idea_text": "Implement $feature with ${config}",
    "backlog_id": "test-123"
  }
}'

# Test 7: Special shell characters
run_test "Special shell characters" \
  '!@#$%^&*()' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Special chars: !@#$%^&*()",
    "open_beads": "None"
  }
}'

# Test 8: Newlines in content
run_test "Multiline content with dollar signs" \
  $'Line 1: $VAR\nLine 2: ${OTHER}' \
  '{
  "stage": "plan",
  "inputs": {
    "spec_name": "test",
    "spec_path": ".gromit/specs/test.md",
    "spec_content": "Line 1: $VAR\nLine 2: ${OTHER}",
    "open_beads": "None"
  }
}'

# Cleanup
rm -f "$STATE_FILE"

echo
echo "========================================="
echo "Tests run: $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $((TESTS_RUN - TESTS_PASSED))"
echo "========================================="

if [ "$TESTS_PASSED" -eq "$TESTS_RUN" ]; then
  echo "✓ All tests passed!"
  exit 0
else
  echo "✗ Some tests failed"
  exit 1
fi
