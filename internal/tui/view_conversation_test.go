package tui

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/conversation"
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

func TestRenderConversationViewRendersTranscriptRows(t *testing.T) {
	store := &Store{
		Conversation: ConversationState{
			Lifecycle: ConversationLifecycleStreaming,
			Transcript: []ConversationTranscriptRow{
				{
					Type: conversation.EventTypeStream,
					Text: "Hello, how can I help?",
				},
				{
					Type: conversation.EventTypeStream,
					Text: "I'll analyze your code.",
				},
			},
		},
	}

	got := RenderConversationView(store, 0)

	if !strings.Contains(got, "Hello, how can I help?") {
		t.Fatalf("expected transcript text in output, got %q", got)
	}
	if !strings.Contains(got, "I'll analyze your code.") {
		t.Fatalf("expected second transcript text in output, got %q", got)
	}
}
