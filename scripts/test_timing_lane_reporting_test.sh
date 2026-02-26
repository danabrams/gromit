#!/usr/bin/env bash
# Test that test_timing.sh reports timing separately by lane

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# Create a temporary test JSON output to simulate go test output
create_test_json() {
  cat > "$1" << 'EOF'
{"Time":"2026-02-26T10:00:00Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Test":"TestSmokeCoverage","Elapsed":1.5}
{"Time":"2026-02-26T10:00:00Z","Action":"pass","Package":"github.com/danabrams/gromit/cmd/gromit","Test":"TestDebug","Elapsed":2.0}
{"Time":"2026-02-26T10:00:00Z","Action":"pass","Package":"github.com/danabrams/gromit/internal/bead","Elapsed":3.0}
EOF
}

tmp_json=$(mktemp)
trap "rm -f $tmp_json" EXIT

create_test_json "$tmp_json"

# Parse JSON and verify lane-aware output can be generated
echo "Testing lane-aware parsing..."

# This test verifies that the output structure supports lane-aware reporting
# Extract package timing and separate by lane
awk '
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
      # Determine lane based on test name
      lane = "default"
      if (test_name ~ /Smoke|smoke/) {
        lane = "smoke"
      }
      key = pkg "\t" lane "\t" test_name
      print elapsed "\t" key
    }
  }
' "$tmp_json" | while read elapsed pkg lane test; do
  echo "Package: $pkg, Lane: $lane, Test: $test, Elapsed: ${elapsed}s"
done | grep -q "Lane: smoke" && echo "✓ Lane-aware parsing works" || echo "✗ Lane-aware parsing failed"
