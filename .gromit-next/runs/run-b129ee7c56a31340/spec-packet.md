# Add Subtract Function

## spec_id
add-subtract

## Title
Add a Subtract function to the calculator

## Problem
The calculator package only has Add. We need Subtract.

## In-Scope
- Add a `Subtract(a, b int) int` function to `calc/calc.go`
- Add tests for Subtract in `calc/calc_test.go`

## Out-of-Scope
- No changes to the Add function
- No new packages

## Acceptance Criteria
1. `calc.Subtract(5, 3)` returns `2`
2. `calc.Subtract(0, 0)` returns `0`
3. `calc.Subtract(3, 5)` returns `-2`
4. All existing tests continue to pass
5. `go vet ./...` passes
6. `gofmt -l .` produces no output

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go test ./calc/...`
- `go vet ./...`
