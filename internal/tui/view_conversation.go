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
	store.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "=== Conversation Panel%s ===\n", panelFocus(focusedPanel, 0))
	fmt.Fprintf(&b, "Lifecycle: %s\n", lifecycleCopy.String())

	return b.String()
}
