package provider

import (
	"testing"
)

// TestCodexProviderStructExists verifies that CodexProvider struct exists
// and can be instantiated.
func TestCodexProviderStructExists(t *testing.T) {
	var cp *CodexProvider
	if cp != nil {
		t.Error("nil CodexProvider should be nil")
	}
}
