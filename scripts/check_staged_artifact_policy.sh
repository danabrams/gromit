#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "${ROOT_DIR}"

mapfile -t staged_files < <(git diff --cached --name-only --diff-filter=ACMR)

if [[ "${#staged_files[@]}" -eq 0 ]]; then
  echo "[repo-hygiene] no staged files to validate for artifact policy"
  exit 0
fi

declare -a blocked=()
for path in "${staged_files[@]}"; do
  # Allow deterministic curated report artifacts in this path.
  if [[ "${path}" =~ ^\.gromit/reports/curated/ ]]; then
    continue
  fi

  if [[ "${path}" =~ ^\.gromit/(state\.json|stats\.json|interactive-state\.json)$ ]]; then
    blocked+=("${path}")
    continue
  fi
  if [[ "${path}" =~ ^\.gromit/metrics/(iteration_metrics\.jsonl|process_trend\.json)$ ]]; then
    blocked+=("${path}")
    continue
  fi
  if [[ "${path}" =~ ^\.gromit/reports/runs/ ]]; then
    blocked+=("${path}")
    continue
  fi
  # Block timestamped report artifacts (for example: *-20260224T130000Z*, *-20260224-130000*).
  if [[ "${path}" =~ ^\.gromit/reports/.*([0-9]{8}T[0-9]{6}Z|[0-9]{8}[-_][0-9]{6}|[0-9]{8}) ]]; then
    blocked+=("${path}")
    continue
  fi
done

if [[ "${#blocked[@]}" -gt 0 ]]; then
  echo "[repo-hygiene] staged runtime or timestamped artifacts are blocked:" >&2
  printf '  %s\n' "${blocked[@]}" >&2
  echo "[repo-hygiene] move raw outputs to ignored paths and commit only deterministic curated artifacts." >&2
  echo "[repo-hygiene] allowed curated path: .gromit/reports/curated/" >&2
  exit 1
fi

echo "[repo-hygiene] staged artifact policy check passed"
