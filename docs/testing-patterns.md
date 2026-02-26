# Validation error handling test pattern

`TestWrapRefactorValidationError` in the validation package captures how we want to exercise wrapped errors when refactoring validation runners. The table-driven structure drives a column per scenario (deadline exceeded, context canceled, generic validation failures) so that each case reuses the same assertion scaffolding while documenting the intent behind every sub-case.

## Table-driven structure

Each table entry corresponds to an expected root cause and includes the input error, a helpful name, and the expected message fragment. The test iterates over the rows via `t.Run`, which keeps the assertions localized while ensuring no scenarios slip through untested. The pattern also captures any future error variants by adding rows rather than duplicating code.

## Triple assertions per case

For a validation error wrapper, every sub-case performs the same three checks:

1. A nil check confirms that we always return an error instead of inadvertently returning `nil` when wrapping fails. This guard ensures the caller never unexpectedly receives a success path.
2. message content is asserted so that the wrapper emits the expected explanation (e.g., `deadline exceeded` or `canceled`) while still allowing us to ignore noisy metadata like timestamps.
3. An `errors.Is` verification traverses the error chain to guarantee the wrapper preserves the original sentinel or typed error. Without this check, refactors can accidentally drop the original context and break callers that rely on `errors.Is` or `errors.As`.

By combining a table-driven layout with these triple assertions, the test documents why we need comprehensive coverage for every path, even when the surface API looks identical for each error.
