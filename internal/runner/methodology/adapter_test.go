package methodology

import "testing"

func TestGoAdapterAcceptanceAddsTagsAndScopes(t *testing.T) {
    t.Parallel()

    adapter := GoAdapter{}
    commands := []string{"go test ./...", "go vet ./..."}
    got := adapter.Acceptance(commands, []string{"internal/runner"})

    if len(got) != 2 {
        t.Fatalf("GoAdapter.Acceptance returned %d commands, want 2", len(got))
    }
    if got[0] != "go test -tags acceptance ./internal/runner/..." {
        t.Errorf("GoAdapter.Acceptance()[0] = %q, want %q", got[0], "go test -tags acceptance ./internal/runner/...")
    }
    if got[1] != "go vet ./..." {
        t.Errorf("GoAdapter.Acceptance()[1] = %q, want unchanged %q", got[1], "go vet ./...")
    }
}
