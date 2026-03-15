# Add Divide Function

## spec_id
divide-with-docs

## Title
Add a Divide function to the calculator

## Problem
The calculator needs a division function with documented behavior.

## In-Scope
- Add a `Divide(a, b int) float64` function to `calc/calc.go`

## Out-of-Scope
- No new test files
- No changes to existing functions

## Acceptance Criteria
1. `Divide(10, 2)` returns `5.0`
2. `Divide(10, 3)` returns approximately `3.333...`
3. `Divide(10, 0)` returns `0.0` — must not return +Inf or NaN
4. The `func Divide` declaration is preceded by a godoc comment (`// Divide ...`) that documents its behavior including the zero-divisor case

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go build ./...`
- `go vet ./...`
