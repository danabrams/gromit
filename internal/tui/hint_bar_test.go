package tui

import (
	"strings"
	"testing"
)

func TestHintBarBacklogTabNormalState(t *testing.T) {
	hint := HintBar(Tab("backlog"), false, false)

	expectedHints := []string{"r", "v", "x", "q"}
	for _, key := range expectedHints {
		if !strings.Contains(hint, key) {
			t.Fatalf("HintBar for backlog tab missing key %q, got %q", key, hint)
		}
	}
}
