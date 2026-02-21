// Package validate implements Stage 3 of the Gromit pipeline: programmatic validation.
// It runs go test, golangci-lint, auto-fix (gofmt/goimports), and periodic full validation.
// On failure, returns summaries that are fed back into the next execute stage Input.
package validate
