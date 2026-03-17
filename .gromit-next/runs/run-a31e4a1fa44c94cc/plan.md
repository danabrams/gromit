# Plan (Cycle 3)

## t-065

Replace \u2014 Unicode escape with literal em-dash character in validate.go contract failure format string (addresses review:code_quality:warning:internal/next/specloop/stages/validate.go:67). Change `"contract:%s \u2014 %s failed: %s"` to `"contract:%s — %s failed: %s"` so the format visually matches the spec.

## t-066

Rename TestIntegration_ContractFailureTriggersReplan to TestIntegration_ContractFailureTriggersReplan_ReplanStageBypassesWriteContracts and add clarifying comment (addresses review:spec_alignment:warning at line 291 and review:code_quality:warning at line 182). The comment must state: 'Note: write_contracts is not re-run because ReplanStage=validate bypasses it in stage ordering, not because ContractsWritten=true. See TestIntegration_WriteContractsIdempotentOnReplan for AC8 coverage.'

