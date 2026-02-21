.PHONY: build install install-skill lint install-hooks shared-state-guard test-parallel-safe-top5 test-touched test-timing test-acceptance-timing test-profile test-unit test-acceptance test-contract test-e2e test-tagged-harness test-e2e-live test-ci

GOLANGCI_LINT_VERSION := $(shell cat .golangci-version)

build:
	go build -o gromit ./cmd/gromit
	go install ./cmd/gromit

install:
	go install ./cmd/gromit

install-skill: build
	./gromit install-skill

lint:
	./scripts/lint.sh

install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "Installed git hooks path: .githooks"
	@echo "Pinned golangci-lint version: $(GOLANGCI_LINT_VERSION)"

shared-state-guard:
	./scripts/check_shared_state_test_calls.sh

test-parallel-safe-top5: shared-state-guard
	./scripts/verify_top5_parallel_execution.sh

test-touched:
	./scripts/test_touched.sh

test-timing:
	./scripts/test_timing.sh

test-acceptance-timing:
	./scripts/test_acceptance_timing.sh

test-profile:
	go test -json ./internal/runner -count=1 | jq -r 'select(.Action=="pass" and .Test != null) | "\(.Elapsed)\t\(.Test)"' | sort -rn

test-unit:
	go test ./...

test-acceptance:
	go test -tags acceptance ./...

test-contract:
	go test -tags=contract ./test/contracts

test-e2e:
	go test -tags=e2e ./test/e2e

test-tagged-harness:
	./scripts/test_tagged_harness.sh

test-e2e-live:
	./scripts/test_e2e_live.sh

test-ci: test-parallel-safe-top5 test-unit test-acceptance
