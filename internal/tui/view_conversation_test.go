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

func TestRenderConversationViewSeparatesTranscriptAndToolStatus(t *testing.T) {
	store := &Store{
		Conversation: ConversationState{
			Lifecycle: ConversationLifecycleToolWait,
			Transcript: []ConversationTranscriptRow{
				{
					Type: conversation.EventTypeStream,
					Text: "First message",
				},
				{
					Type: conversation.EventTypeStream,
					Text: "Second message",
				},
			},
			ToolIndicators: []ConversationToolIndicator{
				{
					ToolName: "Tool1",
					Status:   "waiting",
				},
			},
		},
	}

	got := RenderConversationView(store, 0)

	// Find positions of transcript and tool status in output
	firstMsgPos := strings.Index(got, "First message")
	secondMsgPos := strings.Index(got, "Second message")
	toolPos := strings.Index(got, "Tool1")

	if firstMsgPos == -1 {
		t.Fatalf("expected 'First message' in output, got %q", got)
	}
	if secondMsgPos == -1 {
		t.Fatalf("expected 'Second message' in output, got %q", got)
	}
	if toolPos == -1 {
		t.Fatalf("expected 'Tool1' in output, got %q", got)
	}

	// Verify order: first message < second message < tool status
	if firstMsgPos > secondMsgPos {
		t.Fatalf("expected 'First message' to appear before 'Second message', got %q", got)
	}
	if secondMsgPos > toolPos {
		t.Fatalf("expected transcript messages to appear before tool indicators, got %q", got)
	}
}
