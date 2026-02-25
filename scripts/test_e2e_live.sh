#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mapfile -t pkgs < <(
  go run ./cmd/test_e2e_live_packages --root="$ROOT_DIR" --tag=e2e_live
)

if [[ ${#pkgs[@]} -eq 0 ]]; then
  echo "No e2e_live test packages found."
  exit 0
fi

echo "Running e2e_live tests for packages:"
printf '  %s\n' "${pkgs[@]}"
go test -tags e2e_live "${pkgs[@]}"
