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

func TestHintBarSpecsTabNormalState(t *testing.T) {
	hint := HintBar(Tab("specs"), false, false)

	expectedHints := []string{"p", "v", "x", "q"}
	for _, key := range expectedHints {
		if !strings.Contains(hint, key) {
			t.Fatalf("HintBar for specs tab missing key %q, got %q", key, hint)
		}
	}
}

func TestHintBarPlansTabNormalState(t *testing.T) {
	hint := HintBar(Tab("plans"), false, false)

	expectedHints := []string{"d", "v", "x", "q"}
	for _, key := range expectedHints {
		if !strings.Contains(hint, key) {
			t.Fatalf("HintBar for plans tab missing key %q, got %q", key, hint)
		}
	}
}

func TestHintBarQueueTabNormalState(t *testing.T) {
	hint := HintBar(Tab("queue"), false, false)

	expectedHints := []string{"v", "q"}
	for _, key := range expectedHints {
		if !strings.Contains(hint, key) {
			t.Fatalf("HintBar for queue tab missing key %q, got %q", key, hint)
		}
	}
}

func TestHintBarRunLoopTabNormalState(t *testing.T) {
	hint := HintBar(Tab("runloop"), false, false)

	if !strings.Contains(hint, "q") {
		t.Fatalf("HintBar for runloop tab missing key %q, got %q", "q", hint)
	}
}
