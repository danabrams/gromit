# Add Multiply Function — No Acceptance Criteria

## spec_id
no-acceptance-criteria

## Title
Add a Multiply function to the calculator

## Problem
The calculator package only has Add and Subtract. We need Multiply.

## In-Scope
- Add a `Multiply(a, b int) int` function to `calc/calc.go`
- Add tests for Multiply in `calc/calc_test.go`

## Out-of-Scope
- No changes to existing functions
- No new packages

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go test ./calc/...`
- `go vet ./...`
