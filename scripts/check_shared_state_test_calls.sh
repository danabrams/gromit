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
SHARED_STATE_CALL_PATTERN='os\.(Chdir|Setenv)\('

if [[ ! -f "$ALLOWLIST_FILE" ]]; then
  echo "Missing allowlist: $ALLOWLIST_FILE"
  exit 1
fi

if ! LC_ALL=C sort -c "$ALLOWLIST_FILE"; then
  echo "Allowlist must be sorted: $ALLOWLIST_FILE"
  exit 1
fi

current_calls_file="$(mktemp)"
new_calls_file="$(mktemp)"
trap 'rm -f "$current_calls_file" "$new_calls_file"' EXIT

collect_guarded_test_calls() {
  rg --glob '*_test.go' --no-heading --no-line-number -o "$SHARED_STATE_CALL_PATTERN" "${TARGET_DIRS[@]}" \
    | LC_ALL=C sort \
    >"$current_calls_file" || true
}

collect_guarded_test_calls
LC_ALL=C comm -13 "$ALLOWLIST_FILE" "$current_calls_file" >"$new_calls_file"

if [[ -s "$new_calls_file" ]]; then
  echo "Found newly introduced shared-state test calls in guarded packages:"
  cat "$new_calls_file"
  echo
  echo "Use t.Setenv(), helper seams, or per-test isolation instead of adding new os.Chdir/os.Setenv usage."
  echo "If a new usage is unavoidable, update $ALLOWLIST_FILE in the same change with rationale."
  exit 1
fi

echo "Shared-state call guard passed (no new os.Chdir/os.Setenv in guarded test trees)."
