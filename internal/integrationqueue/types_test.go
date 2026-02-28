package integrationqueue

import "testing"

func TestEntryValidate(t *testing.T) {
	t.Run("valid entry", func(t *testing.T) {
		entry := Entry{
			Branch:        "gromit/ready",
			SessionID:     "session",
			OriginCommand: "refine",
			State:         "ready",
			Lane:          "code_lane",
			BaseRef:       "main",
			HeadSHA:       "deadbeef",
		}
		if err := entry.Validate(); err != nil {
			t.Fatalf("Validate() returned %#v, want nil", err)
		}
	})

	t.Run("missing branch", func(t *testing.T) {
		entry := Entry{
			SessionID:     "session",
			OriginCommand: "refine",
			State:         "ready",
			Lane:          "code_lane",
			BaseRef:       "main",
			HeadSHA:       "deadbeef",
		}
		if err := entry.Validate(); err == nil {
			t.Fatalf("Validate() succeeded with missing branch")
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		entry := Entry{
			Branch:        "gromit/ready",
			SessionID:     "session",
			OriginCommand: "refine",
			State:         "nonsense",
			Lane:          "code_lane",
			BaseRef:       "main",
			HeadSHA:       "deadbeef",
		}
		if err := entry.Validate(); err == nil {
			t.Fatalf("Validate() succeeded with invalid state")
		}
	})
}
