package midreview_test

import (
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/runner/pipeline/midreview"
)

func TestFinding_JSONMarshal(t *testing.T) {
	t.Parallel()

	f := midreview.Finding{
		Category: "performance",
		Message:  "Consider optimizing loop",
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}

	var decoded midreview.Finding
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, want nil", err)
	}

	if decoded.Category != f.Category {
		t.Fatalf("Category = %q, want %q", decoded.Category, f.Category)
	}
	if decoded.Message != f.Message {
		t.Fatalf("Message = %q, want %q", decoded.Message, f.Message)
	}
}
