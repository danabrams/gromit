#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUDGET_FILE="${GROMIT_ACCEPTANCE_BUDGET_FILE:-$ROOT_DIR/scripts/test_acceptance_package_budgets.txt}"
DEFAULT_BUDGET="${GROMIT_ACCEPTANCE_DEFAULT_PACKAGE_BUDGET_SEC:-60}"
TOP_N_PACKAGES="${GROMIT_ACCEPTANCE_TIMING_TOP_PACKAGES:-10}"

tmp_json="$(mktemp)"
tmp_pkg="$(mktemp)"
trap 'rm -f "$tmp_json" "$tmp_pkg"' EXIT

echo "Running acceptance tests with timing output..."
go test -tags acceptance -json -vet=off ./... | tee "$tmp_json" >/dev/null

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
  }
  END {
    for (pkg in pkg_elapsed) {
      printf "%.3f\t%s\n", pkg_elapsed[pkg], pkg
    }
  }
' "$tmp_json" >"$tmp_pkg"

echo
echo "Slowest acceptance packages:"
sort -nr "$tmp_pkg" | sed -n "1,${TOP_N_PACKAGES}p"

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
  echo "$violations acceptance package timing budget violation(s) detected."
  exit 1
fi

echo
echo "All acceptance package timing budgets passed."
