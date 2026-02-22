package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/pipeline/execute"
)

// TestTDDPipelineAdapter_ImplementsInterface verifies that TDDPipelineAdapter
// satisfies the execute.TDDCycleRunner interface.
func TestTDDPipelineAdapter_ImplementsInterface(t *testing.T) {
	var a *TDDPipelineAdapter
	if _, ok := any(a).(execute.TDDCycleRunner); !ok {
		t.Fatal("TDDPipelineAdapter does not implement execute.TDDCycleRunner")
	}
}
