#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALLOWLIST_FILE="${GROMIT_SHARED_STATE_ALLOWLIST:-$ROOT_DIR/scripts/shared_state_test_calls.allowlist}"
TARGET_DIRS=(
  "internal/runner"
  "cmd/gromit"
  "internal/provider"
  "test/testutil"
  "internal/bead"
)

if [[ ! -f "$ALLOWLIST_FILE" ]]; then
  echo "Missing allowlist: $ALLOWLIST_FILE"
  exit 1
fi

if ! LC_ALL=C sort -c "$ALLOWLIST_FILE"; then
  echo "Allowlist must be sorted: $ALLOWLIST_FILE"
  exit 1
fi

tmp_current="$(mktemp)"
tmp_new="$(mktemp)"
trap 'rm -f "$tmp_current" "$tmp_new"' EXIT

rg --glob '*_test.go' --no-heading --no-line-number -o 'os\.(Chdir|Setenv)\(' "${TARGET_DIRS[@]}" \
  | LC_ALL=C sort \
  >"$tmp_current" || true

LC_ALL=C comm -13 "$ALLOWLIST_FILE" "$tmp_current" >"$tmp_new"

if [[ -s "$tmp_new" ]]; then
  echo "Found newly introduced shared-state test calls in guarded packages:"
  cat "$tmp_new"
  echo
  echo "Use t.Setenv(), helper seams, or per-test isolation instead of adding new os.Chdir/os.Setenv usage."
  echo "If a new usage is unavoidable, update $ALLOWLIST_FILE in the same change with rationale."
  exit 1
fi

echo "Shared-state call guard passed (no new os.Chdir/os.Setenv in guarded test trees)."
