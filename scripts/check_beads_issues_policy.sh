#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "${ROOT_DIR}"

issues_path=".beads/issues.jsonl"

if ! git diff --cached --name-only --diff-filter=ACMR | grep -qx "${issues_path}"; then
  echo "[repo-hygiene] no staged ${issues_path} changes"
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "[repo-hygiene] jq is required for ${issues_path} policy checks" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

index_raw="${tmp_dir}/index.raw"
index_canonical="${tmp_dir}/index.canonical"
head_raw="${tmp_dir}/head.raw"
head_canonical="${tmp_dir}/head.canonical"

git show ":${issues_path}" >"${index_raw}"
jq -s 'sort_by(.id)[]' "${index_raw}" | jq -cS '.' >"${index_canonical}"

if ! cmp -s "${index_raw}" "${index_canonical}"; then
  echo "[repo-hygiene] staged ${issues_path} must be canonical (sorted by id, compact JSON, stable key order)" >&2
  exit 1
fi

if git cat-file -e "HEAD:${issues_path}" 2>/dev/null; then
  git show "HEAD:${issues_path}" >"${head_raw}"
  jq -s 'sort_by(.id)[]' "${head_raw}" | jq -cS '.' >"${head_canonical}"

  if ! cmp -s "${head_raw}" "${head_canonical}" && ! cmp -s "${head_canonical}" "${index_canonical}"; then
    echo "[repo-hygiene] split normalization from semantic issue edits in ${issues_path}" >&2
    echo "[repo-hygiene] first land canonical normalization-only rewrite, then stage semantic updates separately" >&2
    exit 1
  fi
fi

echo "[repo-hygiene] staged ${issues_path} policy check passed"
