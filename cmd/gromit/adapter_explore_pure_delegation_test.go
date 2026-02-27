package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterFormalization_RefineRendererSignatureMatchesInterface verifies
// that refinePromptRenderer.RenderRefine has the exact signature required by
// pipeline.RefineRenderer interface.
//
// RED: Tests should validate that adapters have proper typed method signatures
// that exactly match their interface contracts.
func TestAdapterFormalization_RefineRendererSignatureMatchesInterface(t *testing.T) {
	t.Parallel()

	// Verify refinePromptRenderer implements RefineRenderer
	var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)

	// Verify method signature using reflection
	adapter := (*refinePromptRenderer)(nil)
	adapterType := reflect.TypeOf(adapter)

	// Find RenderRefine method
	method, ok := adapterType.MethodByName("RenderRefine")
	if !ok {
		t.Fatal("RenderRefine method not found on refinePromptRenderer")
	}

	// Verify signature: RenderRefine(input *RefinePromptInput) (string, error)
	// NumIn should be 2: receiver + input parameter
	if method.Type.NumIn() != 2 {
		t.Errorf("RenderRefine param count: got %d, want 2", method.Type.NumIn())
	}

	// NumOut should be 2: string + error
	if method.Type.NumOut() != 2 {
		t.Errorf("RenderRefine result count: got %d, want 2", method.Type.NumOut())
	}

	// Verify input parameter type
	inputParam := method.Type.In(1)
	expectedInputType := reflect.TypeOf((*pipeline.RefinePromptInput)(nil))
	if inputParam != expectedInputType {
		t.Errorf("RenderRefine input type: got %v, want %v", inputParam, expectedInputType)
	}

	// Verify return types
	stringType := reflect.TypeOf("")
	if method.Type.Out(0) != stringType {
		t.Errorf("RenderRefine return 0: got %v, want string", method.Type.Out(0))
	}

	if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		t.Errorf("RenderRefine return 1: does not implement error interface")
	}

	t.Log("refinePromptRenderer.RenderRefine has correct signature")
}

// TestAdapterFormalization_BeadQueryClientSignatureMatchesInterface verifies
// that beadQueryClientAdapter methods have exact signatures matching the
// pipeline.BeadQueryClient interface.
//
// RED: Validates that adapter method signatures with context.Context parameters
// are properly typed and handle context correctly.
func TestAdapterFormalization_BeadQueryClientSignatureMatchesInterface(t *testing.T) {
	t.Parallel()

	// Verify beadQueryClientAdapter implements BeadQueryClient
	var _ pipeline.BeadQueryClient = (*beadQueryClientAdapter)(nil)

	adapter := (*beadQueryClientAdapter)(nil)
	adapterType := reflect.TypeOf(adapter)

	// Verify CountByStatus signature: CountByStatus(ctx context.Context, status string) (int, error)
	method, ok := adapterType.MethodByName("CountByStatus")
	if !ok {
		t.Fatal("CountByStatus method not found on beadQueryClientAdapter")
	}

	// NumIn should be 3: receiver + context + string
	if method.Type.NumIn() != 3 {
		t.Errorf("CountByStatus param count: got %d, want 3", method.Type.NumIn())
	}

	// Verify context.Context parameter
	ctxParam := method.Type.In(1)
	if !ctxParam.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		t.Errorf("CountByStatus param 1: got %v, want context.Context", ctxParam)
	}

	// Verify string parameter
	if method.Type.In(2).Kind() != reflect.String {
		t.Errorf("CountByStatus param 2: got %v, want string", method.Type.In(2))
	}

	// Verify return types: int, error
	if method.Type.Out(0).Kind() != reflect.Int {
		t.Errorf("CountByStatus return 0: got %v, want int", method.Type.Out(0))
	}

	if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		t.Errorf("CountByStatus return 1: does not implement error interface")
	}

	t.Log("beadQueryClientAdapter.CountByStatus has correct signature with context.Context")
}

// TestAdapterFormalization_AllPrompRendererSignaturesMatch verifies that all prompt
// renderer adapters (refine, plan, decompose, review, explore) have method signatures
// that exactly match their interface contracts without extra parameters or return values.
//
// RED: This test documents the contract that all adapter render methods must follow
// a simple (input) -> (string, error) signature pattern.
func TestAdapterFormalization_AllPrompRendererSignaturesMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		adapter           interface{}
		methodName        string
		expectedParamType reflect.Type
	}{
		{
			name:              "refinePromptRenderer.RenderRefine",
			adapter:           (*refinePromptRenderer)(nil),
			methodName:        "RenderRefine",
			expectedParamType: reflect.TypeOf((*pipeline.RefinePromptInput)(nil)),
		},
		{
			name:              "planPromptRenderer.RenderPlan",
			adapter:           (*planPromptRenderer)(nil),
			methodName:        "RenderPlan",
			expectedParamType: reflect.TypeOf((*pipeline.PlanPromptInput)(nil)),
		},
		{
			name:              "decomposePromptRenderer.RenderDecompose",
			adapter:           (*decomposePromptRenderer)(nil),
			methodName:        "RenderDecompose",
			expectedParamType: reflect.TypeOf((*pipeline.DecomposePromptInput)(nil)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapterType := reflect.TypeOf(tt.adapter)
			method, ok := adapterType.MethodByName(tt.methodName)
			if !ok {
				t.Fatalf("%s method not found", tt.methodName)
			}

			// Signature should be: method(input *Type) (string, error)
			// NumIn = 2 (receiver + input)
			if method.Type.NumIn() != 2 {
				t.Errorf("param count: got %d, want 2", method.Type.NumIn())
			}

			// Verify input type
			if method.Type.In(1) != tt.expectedParamType {
				t.Errorf("input type: got %v, want %v", method.Type.In(1), tt.expectedParamType)
			}

			// NumOut = 2 (string + error)
			if method.Type.NumOut() != 2 {
				t.Errorf("result count: got %d, want 2", method.Type.NumOut())
			}

			// Verify return types
			if method.Type.Out(0).Kind() != reflect.String {
				t.Errorf("return type 0: got %v, want string", method.Type.Out(0))
			}

			if !method.Type.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				t.Errorf("return type 1: does not implement error")
			}
		})
	}

	t.Log("All prompt renderer adapters have matching interface signatures")
}
