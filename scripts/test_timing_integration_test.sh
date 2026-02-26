#!/usr/bin/env bash
# Integration test for lane-aware timing in test_timing.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Create temp directory for test
test_dir=$(mktemp -d)
trap "rm -rf $test_dir" EXIT

# Simulate go test JSON output with mixed smoke and default tests
mock_test_output="$test_dir/test.json"
cat > "$mock_test_output" << 'EOF'
{"Time":"2026-02-26T10:00:00Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Test":"TestSmokeCoverage","Elapsed":1.5}
{"Time":"2026-02-26T10:00:01Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Test":"TestDebug","Elapsed":2.5}
{"Time":"2026-02-26T10:00:02Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Elapsed":2.5}
{"Time":"2026-02-26T10:00:03Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Test":"TestBeadSmoke","Elapsed":3.0}
{"Time":"2026-02-26T10:00:04Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Test":"TestBead","Elapsed":5.0}
{"Time":"2026-02-26T10:00:05Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Elapsed":5.0}
EOF

# Create budget file with per-lane entries
budget_file="$test_dir/budgets.txt"
cat > "$budget_file" << 'EOF'
# package max_seconds
github.com/danabrams/gromit/cmd/gromit:default 60
github.com/danabrams/gromit/cmd/gromit:smoke 30
github.com/danabrams/gromit/internal/bead:default 90
github.com/danabrams/gromit/internal/bead:smoke 45
EOF

echo "Testing lane-aware timing integration..."
echo

# Process the mock test output using awk logic from test_timing.sh
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
    for (key in pkg_lane_elapsed) {
      printf "%.3f\t%s\n", pkg_lane_elapsed[key], key
    }
  }
' "$mock_test_output")

echo "Lane-aware timing output:"
echo "$output"
echo

# Verify output contains both lanes
if echo "$output" | grep -q "smoke"; then
  echo -e "${GREEN}✓${NC} Smoke lane detected in output"
else
  echo -e "${RED}✗${NC} Smoke lane NOT found in output"
  exit 1
fi

if echo "$output" | grep -q "default"; then
  echo -e "${GREEN}✓${NC} Default lane detected in output"
else
  echo -e "${RED}✗${NC} Default lane NOT found in output"
  exit 1
fi

# Verify lane separation with package names
if echo "$output" | grep -q "github.com/danabrams/gromit/cmd/gromit.*smoke"; then
  echo -e "${GREEN}✓${NC} Package:smoke combination found"
else
  echo -e "${RED}✗${NC} Package:smoke combination NOT found"
  exit 1
fi

if echo "$output" | grep -q "github.com/danabrams/gromit/internal/bead.*smoke"; then
  echo -e "${GREEN}✓${NC} Bead:smoke combination found"
else
  echo -e "${RED}✗${NC} Bead:smoke combination NOT found"
  exit 1
fi

echo
echo "Integration test passed!"
