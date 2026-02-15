#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUDGET_FILE="${GROMIT_TEST_BUDGET_FILE:-$ROOT_DIR/scripts/test_package_budgets.txt}"
DEFAULT_BUDGET="${GROMIT_TEST_DEFAULT_PACKAGE_BUDGET_SEC:-45}"
TOP_N_TESTS="${GROMIT_TEST_TIMING_TOP_N:-10}"
TOP_N_PACKAGES="${GROMIT_TEST_TIMING_TOP_PACKAGES:-10}"

tmp_json="$(mktemp)"
tmp_pkg="$(mktemp)"
tmp_test="$(mktemp)"
trap 'rm -f "$tmp_json" "$tmp_pkg" "$tmp_test"' EXIT

echo "Running go test with timing output..."
go test -json -vet=off ./... | tee "$tmp_json" >/dev/null

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
    if (pkg != "" && test_name == "") {
      if (elapsed > pkg_elapsed[pkg]) {
        pkg_elapsed[pkg] = elapsed
      }
    }
    if (pkg != "" && test_name != "") {
      key = pkg " " test_name
      test_elapsed[key] = elapsed
    }
  }
  END {
    for (pkg in pkg_elapsed) {
      printf "%.3f\t%s\n", pkg_elapsed[pkg], pkg
    }
    print "---"
    for (key in test_elapsed) {
      printf "%.3f\t%s\n", test_elapsed[key], key
    }
  }
' "$tmp_json" | awk '
  BEGIN { section = "pkg" }
  $0 == "---" { section = "test"; next }
  section == "pkg" { print > "'"$tmp_pkg"'" }
  section == "test" { print > "'"$tmp_test"'" }
'

echo
echo "Slowest packages:"
sort -nr "$tmp_pkg" | sed -n "1,${TOP_N_PACKAGES}p"

echo
echo "Slowest tests:"
sort -nr "$tmp_test" | sed -n "1,${TOP_N_TESTS}p"

declare -A budgets
if [[ -f "$BUDGET_FILE" ]]; then
  while read -r pkg max_sec; do
    [[ -z "${pkg:-}" ]] && continue
    [[ "${pkg:0:1}" == "#" ]] && continue
    budgets["$pkg"]="$max_sec"
  done <"$BUDGET_FILE"
fi

violations=0
while IFS=$'\t' read -r elapsed pkg; do
  budget="${budgets[$pkg]:-$DEFAULT_BUDGET}"
  if awk -v e="$elapsed" -v b="$budget" 'BEGIN { exit !(e > b) }'; then
    echo "Budget exceeded: $pkg took ${elapsed}s (budget ${budget}s)"
    violations=$((violations + 1))
  fi
done <"$tmp_pkg"

if [[ "$violations" -gt 0 ]]; then
  echo
  echo "$violations package timing budget violation(s) detected."
  exit 1
fi

echo
echo "All package timing budgets passed."
