#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REQUIRED_VERSION="$(cat "${ROOT_DIR}/.golangci-version")"
GOLANGCI_BIN="${GOLANGCI_BIN:-$(go env GOPATH)/bin/golangci-lint}"

if [[ ! -x "${GOLANGCI_BIN}" ]]; then
  echo "golangci-lint not found at ${GOLANGCI_BIN}"
  echo "Install pinned version with:"
  echo "  GOBIN=\"$(go env GOPATH)/bin\" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${REQUIRED_VERSION}"
  exit 1
fi

VERSION_OUTPUT="$("${GOLANGCI_BIN}" --version 2>&1 || true)"
if [[ "${VERSION_OUTPUT}" != *"version ${REQUIRED_VERSION}"* ]]; then
  echo "golangci-lint version mismatch"
  echo "  required: ${REQUIRED_VERSION}"
  echo "  current:  ${VERSION_OUTPUT}"
  echo "Install pinned version with:"
  echo "  GOBIN=\"$(go env GOPATH)/bin\" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${REQUIRED_VERSION}"
  exit 1
fi

export GOCACHE="${GOCACHE:-/tmp/gocache}"
export GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}"

cd "${ROOT_DIR}"
"${GOLANGCI_BIN}" run ./internal/runner/...
