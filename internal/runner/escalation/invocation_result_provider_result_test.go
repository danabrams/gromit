package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestInvocationResult_StoresProviderResult(t *testing.T) {
	expected := &provider.Result{Success: true}
	result := &runtypes.InvocationResult{ProviderResult: expected}
	if result.ProviderResult != expected {
		t.Fatalf("ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
}
