#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "Running contract-tagged harness suite"
go test -tags=contract ./test/contracts

echo "Running e2e-tagged harness suite"
go test -tags=e2e ./test/e2e
