---
id: explore-command-delegation-test
source_ideas: []
created: 2026-02-26
epic: test-quality
---

# Explore Command Delegation Unit Test

## Problem

The `explore` CLI command has no unit test asserting that it correctly delegates to `pipeline.Explore(ctx, input)` with the right fields. Without this coverage, wiring bugs between the CLI flag parsing and the pipeline call are invisible until manual testing.

## Approach

- Create a mock pipeline that records calls to `Explore(ctx, input)` and returns a fixed result
- Inject the mock into the explore command's pipeline dependency (follow the pattern used by other command delegation tests in `cmd/gromit/`)
- Assert that `Explore` is called exactly once with the correct `input` fields matching the CLI flags provided
- Assert that the command reports the created artifact path from the mock's return value
- Test should not require any filesystem state or subprocess execution

## Files to Change

- `cmd/gromit/explore_test.go` — add `TestExploreCommand_DelegatesToPipeline` using a mock pipeline

## Acceptance Criteria

- Test creates a mock pipeline implementing the explore pipeline interface
- Mock records the `input` argument passed to `Explore`
- Test invokes the explore command with specific CLI flags and asserts the mock was called with matching fields
- Test asserts the command output references the artifact path returned by the mock
- Test passes with `go test ./cmd/gromit/...` without filesystem or subprocess dependencies
