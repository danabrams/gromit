#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mapfile -t pkgs < <(
  rg -l '^//go:build e2e_live' --glob '*_test.go' . \
    | xargs -r -n1 dirname \
    | sed 's#^\./#./#' \
    | sort -u
)

if [[ ${#pkgs[@]} -eq 0 ]]; then
  echo "No e2e_live test packages found."
  exit 0
fi

echo "Running e2e_live tests for packages:"
printf '  %s\n' "${pkgs[@]}"
go test -tags e2e_live "${pkgs[@]}"
