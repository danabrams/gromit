package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestInvocationResult_StoresProviderResult(t *testing.T) {
	expected := &provider.Result{Success: true}
	result := &InvocationResult{ProviderResult: expected}
	if result.ProviderResult != expected {
		t.Fatalf("ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
}
