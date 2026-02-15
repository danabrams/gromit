.PHONY: build install install-skill lint install-hooks

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
