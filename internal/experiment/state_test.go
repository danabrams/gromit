package experiment

import "testing"

func TestLoadStateReturnsZeroStateWhenMissing(t *testing.T) {
    dir := t.TempDir()

    state, err := LoadState(dir, "exp-1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if state == nil {
        t.Fatalf("expected non-nil state")
    }
    if len(state.Arms) != 0 {
        t.Fatalf("expected zero arms for missing state, got %d", len(state.Arms))
    }
}
