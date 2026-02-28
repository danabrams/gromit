package integrationqueue

import "testing"

func TestEntryValidateRejectsLastErrorMessageWithoutCode(t *testing.T) {
    entry := validEntryForValidationTest()
    entry.LastErrorMessage = "something broke"

    if err := entry.Validate(); err == nil {
        t.Fatal("expected validation error when last error message is set without code")
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
