# Acceptance Tests for Typed CLI Adapters (gromit-20iy)

This document describes the acceptance tests for updating CLI adapters to use typed signatures, eliminating `interface{}` returns and map construction.

## Test Overview

These tests verify the acceptance criteria from spec: `pipeline-concrete-types`

### Tests That Should FAIL Until Implementation

These tests verify **new behavior** that doesn't exist yet:

#### 1. `TestPromptRendererAdapter_UsesTypedInputs` (cli_adapters_typed_signatures_test.go)
- **Location**: cmd/gromit/cli_adapters_typed_signatures_test.go:10
- **Verifies**: cliPromptRenderer methods take typed pipeline input structs
- **Current State**: FAILS - RenderRefine, RenderPlan, RenderDecompose, RenderExplore still use `interface{}`
- **Expected Behavior**: Methods should take `*pipeline.RefinePromptInput`, `*pipeline.PlanPromptInput`, etc.
- **Why It Fails**: PromptRenderer interface still uses `interface{}` parameters

#### 2. `TestPipelineInterfaces_AllTypedSignatures` (cli_adapters_typed_signatures_test.go:48)
- **Location**: cmd/gromit/cli_adapters_typed_signatures_test.go:48
- **Verifies**: pipeline.go interface definitions use typed signatures
- **Current State**: FAILS - PromptRenderer interface contains `interface{}` parameters
- **Expected Behavior**: No `interface{}` in any pipeline interface
- **Why It Fails**: PromptRenderer not yet updated to typed signatures

#### 3. `TestPipelinePromptInputTypes_Exist` (cli_adapters_typed_signatures_test.go:80)
- **Location**: cmd/gromit/cli_adapters_typed_signatures_test.go:80
- **Verifies**: Required prompt input types are defined in pipeline.go
- **Current State**: FAILS - RefinePromptInput, PlanPromptInput, DecomposePromptInput, ExplorePromptInput don't exist
- **Expected Behavior**: All four types defined as structs in pipeline.go
- **Why It Fails**: Types have not been created yet

#### 4. `TestPromptRenderer_TakesTypedInput` (typed_interfaces_behavioral_test.go - COMMENTED OUT)
- **Location**: internal/pipeline/typed_interfaces_behavioral_test.go:66
- **Verifies**: PromptRenderer can be used with typed inputs without type assertions
- **Current State**: COMMENTED OUT - won't compile until types exist
- **Expected Behavior**: Mock renderer accepts typed inputs, returns strings
- **Why It's Commented**: RefinePromptInput, PlanPromptInput, etc. don't exist yet
- **Action Required**: Uncomment once types are added

### Tests That PASS (Verify Completed Work)

These tests verify that **already-implemented** parts work correctly:

#### 5. `TestClaudeClientAdapter_ConstructsTypedStruct` (decompose_adapters_test.go)
- **Location**: cmd/gromit/decompose_adapters_test.go:9
- **Verifies**: claudeClientAdapter.Run constructs &pipeline.ClaudeRunResult{}
- **Current State**: PASSES ✓
- **What It Confirms**: Adapter already uses typed return, not map[string]interface{}

#### 6. `TestBeadClientAdapter_ConstructsTypedStruct` (decompose_adapters_test.go)
- **Location**: cmd/gromit/decompose_adapters_test.go:31
- **Verifies**: beadClientAdapter methods construct &pipeline.BeadInfo{}
- **Current State**: PASSES ✓
- **What It Confirms**: All bead adapter methods return typed structs

#### 7. `TestAdapterFile_ImportsTypedPipeline` (decompose_adapters_test.go)
- **Location**: cmd/gromit/decompose_adapters_test.go:61
- **Verifies**: adapters.go imports and uses pipeline types correctly
- **Current State**: PASSES ✓
- **What It Confirms**: Proper imports and type usage

#### 8. `TestAdapterSimplification_NoMapConstruction` (decompose_adapters_test.go)
- **Location**: cmd/gromit/decompose_adapters_test.go:86
- **Verifies**: No intermediate map construction in adapters
- **Current State**: PASSES ✓
- **What It Confirms**: Direct struct construction, no map[string]interface{}

#### 9. `TestAdapters_NoMapConstructionForPrompts` (cli_adapters_typed_signatures_test.go)
- **Location**: cmd/gromit/cli_adapters_typed_signatures_test.go:112
- **Verifies**: No map construction for prompt data in cliPromptRenderer
- **Current State**: PASSES ✓
- **What It Confirms**: Renderer section clean of map constructions

#### 10. `TestDecomposeWorkflow_NoReflectImport` (cli_adapters_typed_signatures_test.go)
- **Location**: cmd/gromit/cli_adapters_typed_signatures_test.go:126
- **Verifies**: decompose.go doesn't import reflect package
- **Current State**: PASSES ✓
- **What It Confirms**: Reflection-based extraction removed

#### 11. `TestDecomposeWorkflow_NoTypeAssertions` (cli_adapters_typed_signatures_test.go)
- **Location**: cmd/gromit/cli_adapters_typed_signatures_test.go:139
- **Verifies**: No .(map[string]interface{}) type assertions in decompose.go
- **Current State**: PASSES ✓
- **What It Confirms**: extractBeadID function deleted, direct field access used

#### 12. `TestClaudeClient_ReturnsTypedResult` (typed_interfaces_behavioral_test.go)
- **Location**: internal/pipeline/typed_interfaces_behavioral_test.go:8
- **Verifies**: ClaudeClient.Run returns typed ClaudeRunResult
- **Current State**: PASSES ✓
- **What It Confirms**: Direct field access without type assertions

#### 13. `TestBeadClient_ReturnsTypedInfo` (typed_interfaces_behavioral_test.go)
- **Location**: internal/pipeline/typed_interfaces_behavioral_test.go:44
- **Verifies**: BeadClient methods return typed BeadInfo
- **Current State**: PASSES ✓
- **What It Confirms**: No extractBeadID needed, direct ID access

#### 14-17. Integration Tests (adapter_integration_typed_test.go)
- **Location**: cmd/gromit/adapter_integration_typed_test.go
- **Tests**:
  - TestClaudeAdapter_IntegrationWithTypedResult
  - TestBeadAdapter_IntegrationWithTypedResult
  - TestWorkflowUsage_NoTypeAssertions
  - TestCompileTimeTypeSafety
- **Current State**: ALL PASS ✓
- **What They Confirm**: Complete adapter chain works with typed results, no type assertions needed

## Summary

**Total Tests**: 17
**Passing**: 13 (verify completed work - ClaudeClient and BeadClient adapters)
**Failing**: 3 (verify missing work - PromptRenderer typed signatures)
**Commented Out**: 1 (won't compile until types exist - behavioral test)

## Acceptance Criteria Coverage

From spec `pipeline-concrete-types`:

✅ ClaudeClient.Run() returns (*ClaudeRunResult, error) - VERIFIED by tests 5, 12, 14
✅ BeadClient methods return (*BeadInfo, error) - VERIFIED by tests 6, 13, 15
❌ PromptRenderer methods take typed input structs - NOT YET (tests 1, 2, 3, 4)
✅ extractBeadID function deleted - VERIFIED by test 11
✅ No reflect import in decompose.go - VERIFIED by test 10
✅ No .(map[string]interface{}) assertions - VERIFIED by test 11
✅ Adapters construct typed structs not maps - VERIFIED by tests 5, 6, 8, 9
✅ go build ./... compiles - VERIFIED (main code builds)
✅ Test mocks updated - VERIFIED by tests 12-17

## Running Tests

Run all adapter tests:
```bash
go test ./cmd/gromit -run "Adapter|Typed" -v
```

Run only failing tests (remaining work):
```bash
go test ./cmd/gromit -run "TestPromptRendererAdapter_UsesTypedInputs|TestPipelineInterfaces_AllTypedSignatures|TestPipelinePromptInputTypes_Exist" -v
```

Run only passing tests (completed work):
```bash
go test ./cmd/gromit -run "TestClaudeClientAdapter|TestBeadClientAdapter|TestAdapterFile|TestAdapterSimplification|TestDecomposeWorkflow" -v
```

## What Implementation Needs to Do

To make all tests pass:

1. **Add types to internal/pipeline/pipeline.go**:
   ```go
   type RefinePromptInput struct { ... }
   type PlanPromptInput struct { ... }
   type DecomposePromptInput struct { ... }
   type ExplorePromptInput struct { ... }
   ```

2. **Update PromptRenderer interface**:
   ```go
   type PromptRenderer interface {
       RenderRefine(input *RefinePromptInput) (string, error)
       RenderPlan(input *PlanPromptInput) (string, error)
       RenderDecompose(input *DecomposePromptInput) (string, error)
       RenderExplore(input *ExplorePromptInput) (string, error)
       RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) // already typed
   }
   ```

3. **Update cliPromptRenderer in cmd/gromit/review.go**:
   - Change method signatures to match updated interface
   - Construct typed input structs instead of using interface{}

4. **Uncomment TestPromptRenderer_TakesTypedInput**:
   - Once types exist, uncomment the test in typed_interfaces_behavioral_test.go
   - Verify it passes
