#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TARGET_PACKAGES=(
  "./internal/runner"
  "./cmd/gromit"
  "./internal/provider"
  "./test/testutil"
  "./internal/bead"
)

DEFAULT_PARALLELISM="${GROMIT_TOP5_PARALLELISM:-8}"
PARALLELISM="${GROMIT_TOP5_PARALLELISM:-${DEFAULT_PARALLELISM}}"

if [[ ! "$PARALLELISM" =~ ^[1-9][0-9]*$ ]]; then
  echo "Invalid GROMIT_TOP5_PARALLELISM=$PARALLELISM; must be positive integer"
  exit 1
fi

echo "Validating parallel-safe execution for top-5 test trees (parallel=$PARALLELISM)"
echo "Shuffling test order to surface shared global-state coupling..."

cd "$ROOT_DIR"
go test -count=1 -parallel="$PARALLELISM" -shuffle=on "${TARGET_PACKAGES[@]}"
