# Smoke Coverage Matrix (Consolidated)

| case | decision | rationale | destination |
| --- | --- | --- | --- |
| cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd | keep | Covers critical end-to-end agent override wiring from CLI flag through process launch. | - |
| cmd/gromit/debug_agent_acceptance_test.go:TestDebugChooseAgentUsesPicker | move | Picker rendering and selection flow is behavior-level logic and does not require full acceptance coverage. | cmd/gromit/debug_agent_test.go:TestDebugChooseAgentUsesPicker_Reclassified |
| cmd/gromit/debug_agent_acceptance_test.go:TestDebugPhaseConfigUsesAgent | move | Phase-based agent resolution can be validated deterministically in unit tests without end-to-end process execution. | cmd/gromit/debug_agent_test.go:TestDebugPhaseConfigUsesAgent_Reclassified |
| cmd/gromit/explore_codex_help_acceptance_test.go:TestCmdSmoke_ExploreAgentSelectionEndToEnd | keep | Verifies critical end-to-end explore invocation with explicit agent override and prompt forwarding. | - |
| cmd/gromit/explore_codex_help_acceptance_test.go:TestExplorePhaseConfigSelectsAgent | move | Explore phase-configured agent selection is deterministic command behavior better covered in focused unit tests. | cmd/gromit/explore_agent_test.go:TestExplorePhaseConfigSelectsAgent_Reclassified |
| cmd/gromit/review_spec_validation_acceptance_test.go:TestCmdSmoke_ReviewSpecValidationEndToEnd | keep | Retains high-value failure-path E2E coverage for strict spec validation and suggestion output. | - |
| internal/runner/validation_extraction_acceptance_test.go:TestRunnerSmoke_RunSingleBeadHappyPath | keep | Preserves core end-to-end success path that validates runner wiring from construction through a full run invocation. | - |
| internal/runner/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier | keep | Retains high-value E2E failure-path coverage for timeout-triggered retry with tier escalation behavior. | - |
| internal/runner/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd | keep | Covers critical end-to-end merge-failure warning path to ensure run loop continues under configured warn mode. | - |
