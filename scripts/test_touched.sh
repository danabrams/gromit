#!/usr/bin/env bash
set -euo pipefail

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Not inside a git repository."
  exit 1
fi

mapfile -t packages < <(
  git diff --name-only --diff-filter=ACMRTUXB HEAD -- '*.go' \
    | xargs -r -n1 dirname \
    | sed 's#^\.$##' \
    | sed 's#^\./##' \
    | sed '/^$/d' \
    | sort -u \
    | awk '{print "./" $0 "/..."}'
)

if [[ ${#packages[@]} -eq 0 ]]; then
  echo "No changed Go packages detected. Skipping touched-package test run."
  echo "Run full suite with: go test -vet=off ./..."
  exit 0
fi

echo "Running tests for touched packages:"
printf '  %s\n' "${packages[@]}"
go test -vet=off "${packages[@]}"
