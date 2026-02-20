#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 4 ]]; then
  echo "Usage: $0 <go-test-jsonl> [output-json] [top_overall] [top_per_package]" >&2
  exit 1
fi

input_jsonl="$1"
output_json="${2:-${input_jsonl%.jsonl}.summary.json}"
top_overall="${3:-25}"
top_per_package="${4:-10}"

if [[ ! -f "$input_jsonl" ]]; then
  echo "Input file not found: $input_jsonl" >&2
  exit 1
fi

jq -s \
  --arg generated_at "$(date -Iseconds)" \
  --arg input "$input_jsonl" \
  --argjson top_overall "$top_overall" \
  --argjson top_per_package "$top_per_package" \
'
def pass_tests:
  [ .[] | select(.Action == "pass" and (.Test != null) and (.Elapsed != null))
    | {package: .Package, test: .Test, elapsed_seconds: .Elapsed} ];

def package_totals:
  [ .[] | select((.Action == "pass" or .Action == "fail") and (.Test == null) and (.Elapsed != null))
    | {package: .Package, status: .Action, elapsed_seconds: .Elapsed} ];

{
  generated_at: $generated_at,
  input: $input,
  top_overall_count: $top_overall,
  top_per_package_count: $top_per_package,
  top_tests_overall: (pass_tests | sort_by(.elapsed_seconds) | reverse | .[0:$top_overall]),
  top_tests_by_package: (
    pass_tests
    | sort_by(.package)
    | group_by(.package)
    | map({
        package: .[0].package,
        tests: (sort_by(.elapsed_seconds) | reverse | .[0:$top_per_package])
      })
  ),
  package_totals: (package_totals | sort_by(.elapsed_seconds) | reverse),
  failed_packages: (
    [ .[] | select(.Action == "fail" and (.Test == null)) | .Package ]
    | unique
  )
}
' "$input_jsonl" > "$output_json"

echo "$output_json"
