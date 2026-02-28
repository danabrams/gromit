package tui

import (
	"strings"
	"testing"
)

func TestRenderConversationViewRendersLifecycleHeader(t *testing.T) {
	store := &Store{
		Conversation: ConversationState{
			Lifecycle: ConversationLifecycleStreaming,
		},
	}

	got := RenderConversationView(store, 0)

	if !strings.Contains(got, "streaming") {
		t.Fatalf("expected lifecycle status 'streaming' in output, got %q", got)
	}
}
