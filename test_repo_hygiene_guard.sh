#!/usr/bin/env bash
set -euo pipefail

# Test that repo-hygiene guard passes with no tracked artifacts
# This is a RED test - it should fail if tracked artifacts exist
make repo-hygiene-guard
