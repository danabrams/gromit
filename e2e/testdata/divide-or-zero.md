# Add Divide Function

## spec_id
divide-or-zero

## Title
Add a Divide function to the calculator

## Problem
The calculator needs a division function.

## In-Scope
- Add a `Divide(a, b int) float64` function to `calc/calc.go`
- The function should compute the quotient of a divided by b

## Out-of-Scope
- No changes to existing functions
- No test files required

## Acceptance Criteria
1. `Divide(10, 2)` returns `5.0`
2. `Divide(10, 3)` returns approximately `3.333...`
3. `Divide(10, 0)` returns `0.0` — must not return +Inf or NaN

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go build ./...`
- `go vet ./...`
