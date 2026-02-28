package integrationqueue

import (
	"reflect"
	"testing"
)

func TestEntryValidate(t *testing.T) {
	t.Run("valid entry", func(t *testing.T) {
		entry := validEntryForValidationTest()
		if err := entry.Validate(); err != nil {
			t.Fatalf("Validate() returned %v, want nil", err)
		}
	})

	failureCases := []struct {
		name   string
		mutate func(*Entry)
	}{
		{
			name: "missing branch",
			mutate: func(e *Entry) {
				e.Branch = ""
			},
		},
		{
			name: "missing session",
			mutate: func(e *Entry) {
				e.SessionID = ""
			},
		},
		{
			name: "missing origin command",
			mutate: func(e *Entry) {
				e.OriginCommand = "   "
			},
		},
		{
			name: "missing lane",
			mutate: func(e *Entry) {
				e.Lane = ""
			},
		},
		{
			name: "missing base ref",
			mutate: func(e *Entry) {
				e.BaseRef = ""
			},
		},
		{
			name: "missing head sha",
			mutate: func(e *Entry) {
				e.HeadSHA = ""
			},
		},
	}

	for _, tt := range failureCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			entry := validEntryForValidationTest()
			tt.mutate(&entry)
			if err := entry.Validate(); err == nil {
				t.Fatalf("Validate() succeeded for %s", tt.name)
			}
		})
	}

	t.Run("invalid state", func(t *testing.T) {
		entry := validEntryForValidationTest()
		entry.State = "unknown"
		if err := entry.Validate(); err == nil {
			t.Fatal("Validate() succeeded with invalid state")
		}
	})

	t.Run("last error message without code", func(t *testing.T) {
		entry := validEntryForValidationTest()
		entry.LastErrorMessage = "something broke"
		if err := entry.Validate(); err == nil {
			t.Fatal("expected validation error when last error message is set without code")
		}
	})

	t.Run("last error code without message", func(t *testing.T) {
		entry := validEntryForValidationTest()
		entry.LastErrorCode = "code"
		if err := entry.Validate(); err == nil {
			t.Fatal("expected validation error when last error code is set without message")
		}
	})
}

func TestEntryValidateDoesNotMutateInput(t *testing.T) {
	entry := validEntryForValidationTest()
	copy := entry
	if err := entry.Validate(); err != nil {
		t.Fatalf("Validate() returned %v, want nil", err)
	}
	if !reflect.DeepEqual(entry, copy) {
		t.Fatalf("entry mutated: got %+v, want %+v", entry, copy)
	}
}

func validEntryForValidationTest() Entry {
	return Entry{
		Branch:        "gromit/ready",
		SessionID:     "session",
		OriginCommand: "ready",
		State:         StateReady,
		Lane:          "code_lane",
		BaseRef:       "main",
		HeadSHA:       "deadbeef",
	}
}
