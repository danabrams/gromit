package event

import "strings"

// WireWorktreeFileSubscriber registers a file subscriber on emitter that writes
// events to <worktree>/.gromit/v2/events.jsonl. It returns a cleanup function
// that unsubscribes and closes the subscriber.
func WireWorktreeFileSubscriber(emitter *Emitter, worktree string) func() {
	if emitter == nil {
		return nil
	}
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return nil
	}

	fs := NewWorktreeFileSubscriber(trimmed)
	unsubscribe := fs.SubscribeTo(emitter)

	return func() {
		unsubscribe()
		_ = fs.Close()
	}
}
