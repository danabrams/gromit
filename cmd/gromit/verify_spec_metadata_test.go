package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSpecGateBeadCreatorWithTrackerClient_LabelsJSON(t *testing.T) {
	t.Parallel()

	mockClient := &mockTrackerClient{}
	creator, err := newSpecGateBeadCreatorWithTrackerClient(mockClient)
	if err != nil {
		t.Fatalf("newSpecGateBeadCreatorWithTrackerClient error: %v", err)
	}

	labels := []string{"spec:alpha", "priority:high"}
	if _, err := creator.Create(context.Background(), "test", "desc", "P1", labels); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if len(mockClient.createdItems) == 0 {
		t.Fatal("expected tracker client Create to be invoked")
	}

	raw := mockClient.createdItems[0].Metadata["labels"]
	if raw == "" {
		t.Fatal("labels metadata is empty")
	}

	var decoded []string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal labels metadata: %v", err)
	}

	if len(decoded) != len(labels) {
		t.Fatalf("decoded labels length = %d, want %d", len(decoded), len(labels))
	}
	for i, want := range labels {
		if decoded[i] != want {
			t.Fatalf("label[%d] = %q, want %q", i, decoded[i], want)
		}
	}
}
