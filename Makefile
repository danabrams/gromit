.PHONY: build install install-skill lint install-hooks test-touched test-timing test-acceptance-timing test-unit test-acceptance test-e2e-live test-ci

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

test-touched:
	./scripts/test_touched.sh

test-timing:
	./scripts/test_timing.sh

test-acceptance-timing:
	./scripts/test_acceptance_timing.sh

test-unit:
	go test ./...

test-acceptance:
	go test -tags acceptance ./...

test-e2e-live:
	./scripts/test_e2e_live.sh

test-ci: test-unit test-acceptance
