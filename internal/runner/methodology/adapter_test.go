package methodology

import "testing"
import "reflect"

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

func TestPassthroughAdapterLeavesCommandsUnchanged(t *testing.T) {
	t.Parallel()

	adapter := PassthroughAdapter{}
	original := []string{"go test ./...", "go vet ./..."}
	touched := []string{"internal/runner"}

	assertUnchanged := func(name string, got []string) {
		if !reflect.DeepEqual(got, original) {
			t.Fatalf("PassthroughAdapter.%s() = %q, want %q", name, got, original)
		}
	}

	assertUnchanged("Acceptance", adapter.Acceptance(original, touched))
	assertUnchanged("TDD", adapter.TDD(original, touched))
	assertUnchanged("Validation", adapter.Validation(original, touched))
}
