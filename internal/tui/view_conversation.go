package tui

import (
	"fmt"
	"strings"
)

// RenderConversationView builds a textual representation of the conversation view from the store.
func RenderConversationView(store *Store, focusedPanel int) string {
	if store == nil {
		return ""
	}

	store.mu.RLock()
	lifecycleCopy := store.Conversation.Lifecycle
	transcriptCopy := append([]ConversationTranscriptRow{}, store.Conversation.Transcript...)
	toolIndicatorsCopy := append([]ConversationToolIndicator{}, store.Conversation.ToolIndicators...)
	store.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "=== Conversation Panel%s ===\n", panelFocus(focusedPanel, 0))
	fmt.Fprintf(&b, "Lifecycle: %s\n", lifecycleCopy.String())

	for _, row := range transcriptCopy {
		if row.Text != "" {
			fmt.Fprintf(&b, "%s\n", row.Text)
		}
	}

	for _, indicator := range toolIndicatorsCopy {
		fmt.Fprintf(&b, "[%s] %s\n", indicator.ToolName, indicator.Status)
	}

	return b.String()
}
