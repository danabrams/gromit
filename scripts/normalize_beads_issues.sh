#!/usr/bin/env bash
set -euo pipefail

issues_file="${1:-.beads/issues.jsonl}"
if [[ ! -f "$issues_file" ]]; then
  echo "normalize_beads_issues: file not found: $issues_file" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "normalize_beads_issues: jq is required" >&2
  exit 1
fi

tmp="$(mktemp)"
cleanup() {
  rm -f "$tmp"
}
trap cleanup EXIT

# Canonicalize key ordering, sort records by ID, and keep one JSON object per line.
jq -s 'sort_by(.id)[]' "$issues_file" | jq -cS '.' >"$tmp"

if cmp -s "$issues_file" "$tmp"; then
  echo "normalize_beads_issues: no changes"
  exit 0
fi

mv "$tmp" "$issues_file"
echo "normalize_beads_issues: normalized $issues_file"
