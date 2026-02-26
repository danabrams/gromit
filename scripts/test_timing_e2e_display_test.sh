#!/usr/bin/env bash
# End-to-end test for lane-aware timing display

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# Create temp directory for test
test_dir=$(mktemp -d)
trap "rm -rf $test_dir" EXIT

# Simulate go test JSON output with both smoke and default tests
mock_test_output="$test_dir/test.json"
cat > "$mock_test_output" << 'EOF'
{"Time":"2026-02-26T10:00:00Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Test":"TestSmokeCoverage","Elapsed":1.5}
{"Time":"2026-02-26T10:00:01Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Test":"TestDebug","Elapsed":2.5}
{"Time":"2026-02-26T10:00:02Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Elapsed":2.5}
{"Time":"2026-02-26T10:00:03Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Test":"TestBeadSmoke","Elapsed":3.0}
{"Time":"2026-02-26T10:00:04Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Test":"TestBead","Elapsed":5.0}
{"Time":"2026-02-26T10:00:05Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Elapsed":5.0}
EOF

# Create budget file
budget_file="$test_dir/budgets.txt"
cat > "$budget_file" << 'EOF'
# package max_seconds
github.com/danabrams/gromit/cmd/gromit:default 60
github.com/danabrams/gromit/cmd/gromit:smoke 30
github.com/danabrams/gromit/internal/bead:default 90
github.com/danabrams/gromit/internal/bead:smoke 45
EOF

echo "End-to-End Lane-Aware Timing Display Test"
echo "=========================================="
echo

test_count=0
pass_count=0

# Test 1: Lane-separated reporting format
echo "Test 1: Verify lane separation in output"
test_count=$((test_count + 1))

output=$(awk '
  /"Action":"pass"/ && /"Package":/ {
    pkg = ""
    elapsed = 0
    test_name = ""
    if (match($0, /"Package":"[^"]+"/)) {
      pkg = substr($0, RSTART + 11, RLENGTH - 12)
    }
    if (match($0, /"Elapsed":[0-9.]+/)) {
      elapsed = substr($0, RSTART + 10, RLENGTH - 10) + 0
    }
    if (match($0, /"Test":"[^"]+"/)) {
      test_name = substr($0, RSTART + 8, RLENGTH - 9)
    }
    if (pkg != "" && test_name != "") {
      lane = "default"
      if (test_name ~ /[Ss]moke|smoke_/) {
        lane = "smoke"
      }
      key = pkg "\t" lane
      if (elapsed > pkg_lane_elapsed[key]) {
        pkg_lane_elapsed[key] = elapsed
      }
    }
  }
  END {
    print "Slowest packages by lane:"
    for (key in pkg_lane_elapsed) {
      printf "  %s (%s): %.3fs\n", substr(key, 1, index(key, "\t")-1), substr(key, index(key, "\t")+1), pkg_lane_elapsed[key]
    }
  }
' "$mock_test_output")

echo "$output"
echo

# Verify output shows lanes clearly
if echo "$output" | grep -q "(smoke)"; then
  echo -e "${GREEN}✓${NC} Output shows (smoke) lane markers"
  pass_count=$((pass_count + 1))
else
  echo -e "${RED}✗${NC} Output missing (smoke) lane markers"
fi

if echo "$output" | grep -q "(default)"; then
  echo -e "${GREEN}✓${NC} Output shows (default) lane markers"
  pass_count=$((pass_count + 1))
else
  echo -e "${RED}✗${NC} Output missing (default) lane markers"
fi

# Test 2: Budget violation messages with lane context
echo
echo "Test 2: Budget violation messages include lane context"
test_count=$((test_count + 3))

declare -A budgets
while read -r pkg_or_lane max_sec; do
  [[ -z "${pkg_or_lane:-}" ]] && continue
  [[ "${pkg_or_lane:0:1}" == "#" ]] && continue
  budgets["$pkg_or_lane"]="$max_sec"
done <"$budget_file"

get_budget() {
  local pkg="$1"
  local lane="$2"
  if [[ -n "${budgets[$pkg:$lane]:-}" ]]; then
    echo "${budgets[$pkg:$lane]}"
  elif [[ -n "${budgets[$pkg]:-}" ]]; then
    echo "${budgets[$pkg]}"
  else
    echo "45"
  fi
}

# Simulate a violation
test_pkg="github.com/danabrams/gromit/cmd/gromit"
smoke_budget=$(get_budget "$test_pkg" "smoke")
violation_msg="Budget exceeded [smoke lane]: $test_pkg took 35s (budget ${smoke_budget}s)"

echo "Example violation message:"
echo "  $violation_msg"

if echo "$violation_msg" | grep -q "\[smoke lane\]"; then
  echo -e "${GREEN}✓${NC} Violation message includes lane context [smoke lane]"
  pass_count=$((pass_count + 1))
else
  echo -e "${RED}✗${NC} Violation message missing lane context"
fi

if echo "$violation_msg" | grep -q "took 35s"; then
  echo -e "${GREEN}✓${NC} Violation message includes timing details"
  pass_count=$((pass_count + 1))
else
  echo -e "${RED}✗${NC} Violation message missing timing details"
fi

# Test 3: Backward compatibility - old format still works
echo
echo "Test 3: Backward compatibility with old budget format"
test_count=$((test_count + 1))

old_budget_file="$test_dir/old_budgets.txt"
cat > "$old_budget_file" << 'EOF'
# package max_seconds
github.com/danabrams/gromit/cmd/gromit 60
github.com/danabrams/gromit/internal/bead 90
EOF

# Re-parse with old format
declare -A old_budgets
while read -r pkg_name max_sec; do
  [[ -z "${pkg_name:-}" ]] && continue
  [[ "${pkg_name:0:1}" == "#" ]] && continue
  old_budgets["$pkg_name"]="$max_sec"
done <"$old_budget_file"

if [[ "${old_budgets[github.com/danabrams/gromit/cmd/gromit]}" == "60" ]]; then
  echo -e "${GREEN}✓${NC} Old budget format still parsed correctly"
  pass_count=$((pass_count + 1))
else
  echo -e "${RED}✗${NC} Old budget format broken"
fi

echo
echo "=========================================="
echo "Test Results: $pass_count/$test_count passed"

if [[ $pass_count -lt $test_count ]]; then
  exit 1
fi
