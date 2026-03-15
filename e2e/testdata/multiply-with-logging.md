# Add Multiply Function With Logging
## spec_id
multiply-with-logging
## Title
Add a Multiply function that logs its inputs
## Problem
The calculator needs a Multiply function that records its inputs for audit purposes.
## In-Scope
- Add a `Multiply(a, b int) int` function to `calc/calc.go`
- The function must record each invocation (inputs and result) to a package-level slice `var AuditLog []string`
- Add tests for Multiply correctness in `calc/calc_test.go`
## Out-of-Scope
- No changes to existing functions
- No external logging libraries
## Acceptance Criteria
1. `calc.Multiply(3, 4)` returns `12`
2. `calc.Multiply(0, 5)` returns `0`
3. After calling `Multiply(3, 4)`, `AuditLog` contains an entry recording the inputs and result
4. All existing tests continue to pass
5. `go vet ./...` passes
## Architectural Constraints
- All code stays in the `calc` package
## Validation
- `go test ./calc/...`
- `go vet ./...`
