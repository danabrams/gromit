#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "$ROOT_DIR"

# Git mode 160000 indicates a gitlink (submodule entry). This repo should not contain them.
gitlinks="$(git ls-files -s | awk '$1=="160000"{print $4}')"
if [[ -n "${gitlinks}" ]]; then
  echo "[repo-hygiene] unexpected gitlink entries detected:" >&2
  echo "${gitlinks}" >&2
  echo "Remove these from git index (for example: git rm --cached <path>)." >&2
  exit 1
fi

# Runtime artifacts and benchmark run outputs must never be tracked.
# Keep only deterministic fixtures under approved fixture paths.
blocked_regex='^(\.dolt/|\.doltcfg/|beads_gromit/|cmd/gromit/\.gromit/interactive-state\.json\.lock$|cmd/gromit/\.gromit/benchmarks/results/)'
blocked="$(git ls-files | grep -E "${blocked_regex}" || true)"
if [[ -n "${blocked}" ]]; then
  echo "[repo-hygiene] unexpected tracked runtime artifacts detected:" >&2
  echo "${blocked}" >&2
  echo "Remove these from git index (for example: git rm --cached <path>)." >&2
  exit 1
fi

echo "[repo-hygiene] no gitlink entries found"
echo "[repo-hygiene] no tracked runtime artifact paths found"
