# Smoke Coverage Matrix (Consolidated)

## Smoke Decision Rubric

Retain an acceptance case as E2E smoke only when it validates a critical success or critical failure outcome. Everything else moves to behavior-level tests so routine logic stays fast, deterministic, and local.

## Matrix Template

Template fields: source case, keep/move, rationale, destination suite/file.

## Destination Conventions

Use `file:testname` (or `file:suite` for suite-level destinations) and keep the path within the relevant domain:

- cmd/gromit destinations live under `cmd/gromit/*_test.go`
- internal/runner destinations live under `internal/runner/*_test.go`

| case | decision | rationale | destination |
| --- | --- | --- | --- |
| cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd | keep | Covers critical end-to-end agent override wiring from CLI flag through process launch. | cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd |
| cmd/gromit/debug_agent_acceptance_test.go:TestDebugChooseAgentUsesPicker | move | Picker rendering and selection flow is behavior-level logic and does not require full acceptance coverage. | cmd/gromit/debug_agent_test.go:TestDebugChooseAgentUsesPicker_Reclassified |
| cmd/gromit/debug_agent_acceptance_test.go:TestDebugPhaseConfigUsesAgent | move | Phase-based agent resolution can be validated deterministically in unit tests without end-to-end process execution. | cmd/gromit/debug_agent_test.go:TestDebugPhaseConfigUsesAgent_Reclassified |
| cmd/gromit/explore_codex_help_acceptance_test.go:TestCmdSmoke_ExploreAgentSelectionEndToEnd | keep | Verifies critical end-to-end explore invocation with explicit agent override and prompt forwarding. | cmd/gromit/explore_codex_help_acceptance_test.go:TestCmdSmoke_ExploreAgentSelectionEndToEnd |
| cmd/gromit/explore_codex_help_acceptance_test.go:TestExplorePhaseConfigSelectsAgent | move | Explore phase-configured agent selection is deterministic command behavior better covered in focused unit tests. | cmd/gromit/explore_agent_test.go:TestExplorePhaseConfigSelectsAgent_Reclassified |
| cmd/gromit/review_spec_validation_acceptance_test.go:TestCmdSmoke_ReviewSpecValidationEndToEnd | keep | Retains high-value failure-path E2E coverage for strict spec validation and suggestion output. | cmd/gromit/review_spec_validation_acceptance_test.go:TestCmdSmoke_ReviewSpecValidationEndToEnd |
| internal/runner/acceptance/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier | keep | Retains high-value E2E failure-path coverage for timeout-triggered retry with tier escalation behavior. | internal/runner/acceptance/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier |
| internal/runner/acceptance/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd | keep | Covers critical end-to-end merge-failure warning path to ensure run loop continues under configured warn mode. | internal/runner/acceptance/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd |
| internal/runner/andon/policy_classification_acceptance_test.go:TestEvaluateFailure_ClassifiesAndSelectsDecisionForAllClasses | move | Policy classification and decision selection are deterministic logic with comprehensive unit coverage. | internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass |
| internal/runner/andon/policy_classification_acceptance_test.go:TestEvaluateFailure_EnforcesL1L2BoundaryAtPublicEntryPoint | move | L1/L2 boundary enforcement is a pure policy rule covered by targeted unit tests. | internal/runner/andon/policy_test.go:TestEvaluateFailure_EnforcesL1ToL2BoundaryAtRetryCap |
| internal/runner/andon/policy_class_aware_selection_acceptance_test.go:TestEvaluateFailure_UsesClassifiedDecisionPathAtPublicEntryPoint | move | Decision path selection per failure class is deterministic logic covered in unit policy tests. | internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass |
| internal/runner/andon/policy_class_aware_selection_acceptance_test.go:TestEvaluateClassifiedFailure_HasExplicitDecisionPathPerFailureClass | move | Explicit class-to-path mappings are validated in unit tests without needing acceptance-level coverage. | internal/runner/andon/policy_test.go:TestEvaluateClassifiedFailure_HasExplicitPathPerClass |
| internal/runner/andon/policy_class_aware_selection_acceptance_test.go:TestEvaluateFailure_UnknownSignalRemainsDeterministicWithWorkflowFallbackPath | move | Unknown-signal fallback determinism is covered by targeted unit tests of the policy evaluator. | internal/runner/andon/policy_test.go:TestEvaluateFailure_UnknownKindUsesDeterministicWorkflowFallbackPath |
| internal/runner/andon/types_acceptance_test.go:TestFailureClasses_CanonicalCatalog | move | Catalog ordering/labels are deterministic data checks already covered in unit tests without end-to-end wiring. | internal/runner/andon/types_test.go:TestAllFailureClasses_CanonicalOrderAndLabels |
| internal/runner/andon/types_acceptance_test.go:TestLevels_CanonicalCatalog | move | Level catalog validation is a pure data invariant that belongs in unit coverage, not acceptance flow. | internal/runner/andon/types_test.go:TestAllAndonLevels_CanonicalOrder |
| internal/runner/andon/types_acceptance_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable_Acceptance | move | Default threshold purity and policy consumption are deterministic and exercised via unit tests. | internal/runner/andon/types_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable |
