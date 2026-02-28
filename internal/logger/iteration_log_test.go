package logger

import (
	"encoding/json"
	"testing"
)

func TestIterationLogGateBlockReasonJSON(t *testing.T) {
	t.Parallel()

	withReason := IterationLog{
		GateBlockReason: "scope",
	}
	gotWithReason := make(map[string]any)
	data, err := json.Marshal(withReason)
	if err != nil {
		t.Fatalf("json.Marshal(withReason) error = %v", err)
	}
	if err := json.Unmarshal(data, &gotWithReason); err != nil {
		t.Fatalf("json.Unmarshal(withReason) error = %v", err)
	}
	if got, ok := gotWithReason["gate_block_reason"]; !ok {
		t.Fatalf("expected gate_block_reason key to be present")
	} else if got != "scope" {
		t.Fatalf("gate_block_reason = %v, want %q", got, "scope")
	}

	withoutReason := IterationLog{}
	gotWithoutReason := make(map[string]any)
	data, err = json.Marshal(withoutReason)
	if err != nil {
		t.Fatalf("json.Marshal(withoutReason) error = %v", err)
	}
	if err := json.Unmarshal(data, &gotWithoutReason); err != nil {
		t.Fatalf("json.Unmarshal(withoutReason) error = %v", err)
	}
	if _, ok := gotWithoutReason["gate_block_reason"]; ok {
		t.Fatalf("gate_block_reason should be omitted when empty")
	}
}
