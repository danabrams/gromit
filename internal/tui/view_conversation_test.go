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

func TestRenderConversationViewRendersToolIndicators(t *testing.T) {
	store := &Store{
		Conversation: ConversationState{
			Lifecycle: ConversationLifecycleToolWait,
			Transcript: []ConversationTranscriptRow{
				{
					Type: conversation.EventTypeStream,
					Text: "I'm processing your request",
				},
			},
			ToolIndicators: []ConversationToolIndicator{
				{
					ToolName: "CodeAnalyzer",
					Status:   "waiting",
				},
				{
					ToolName: "FileReader",
					Status:   "waiting",
				},
			},
		},
	}

	got := RenderConversationView(store, 0)

	if !strings.Contains(got, "CodeAnalyzer") {
		t.Fatalf("expected tool name 'CodeAnalyzer' in output, got %q", got)
	}
	if !strings.Contains(got, "waiting") {
		t.Fatalf("expected tool status 'waiting' in output, got %q", got)
	}
	if !strings.Contains(got, "FileReader") {
		t.Fatalf("expected tool name 'FileReader' in output, got %q", got)
	}
}

func TestRenderConversationViewRendersInputArea(t *testing.T) {
	store := &Store{
		Conversation: ConversationState{
			Lifecycle: ConversationLifecycleStreaming,
		},
	}

	got := RenderConversationView(store, 0)

	if !strings.Contains(got, ">") || !strings.Contains(got, "Input:") {
		t.Fatalf("expected input area (with '>' or 'Input:') in output, got %q", got)
	}
}
