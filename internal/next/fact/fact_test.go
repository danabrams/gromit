package fact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{Declared, "declared"},
		{Observed, "observed"},
		{Inferred, "inferred"},
	}
	for _, tt := range tests {
		if got := tt.cat.String(); got != tt.want {
			t.Errorf("Category(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestNewFact(t *testing.T) {
	f := New("test-001", Observed, "go.mod declares Go 1.22", "go-module-extractor")
	if f.ID != "test-001" {
		t.Errorf("ID = %q, want %q", f.ID, "test-001")
	}
	if f.Category != Observed {
		t.Errorf("Category = %v, want Observed", f.Category)
	}
	if f.Source != "go-module-extractor" {
		t.Errorf("Source = %q, want %q", f.Source, "go-module-extractor")
	}
	if f.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestCategory_UnmarshalJSON_Unknown(t *testing.T) {
	var c Category
	err := json.Unmarshal([]byte(`"bogus"`), &c)
	if err == nil {
		t.Fatal("expected error for unknown category, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error %q should mention the invalid value", err.Error())
	}
}

func TestCategory_JSONRoundTrip(t *testing.T) {
	f := New("test-001", Observed, "test content", "test-source")
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"observed"`) {
		t.Errorf("expected JSON to contain \"observed\", got %s", data)
	}

	var f2 Fact
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if f2.Category != Observed {
		t.Errorf("Category = %v, want Observed", f2.Category)
	}
}
