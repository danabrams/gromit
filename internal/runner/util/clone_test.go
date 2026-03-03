package util

import "testing"

func TestCloneStringSliceReturnsNilForNilInput(t *testing.T) {
    if got := cloneStringSlice(nil); got != nil {
        t.Fatalf("expected nil, got %v", got)
    }
}
