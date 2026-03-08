package event

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/cli"
	"github.com/danabrams/gromit/internal/events/stream"
)

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

// StartEventLogSubscriber wires a subscriber that appends typed events to the
// supplied JSONL path and returns a cleanup callback that unsubscribes and closes
// the underlying file handle.
func StartEventLogSubscriber(emitter *Emitter, eventsPath string) func() {
	if emitter == nil {
		return nil
	}
	trimmed := strings.TrimSpace(eventsPath)
	if trimmed == "" {
		return nil
	}
	fs := NewFileSubscriber(trimmed)
	unsubscribe := fs.SubscribeTo(emitter)
	return func() {
		unsubscribe()
		_ = fs.Close()
	}
}

// StartWorktreeEventLogSubscriber wires a subscriber that appends typed events to
// <worktree>/.gromit/v2/events.jsonl, preserving any existing log records.
func StartWorktreeEventLogSubscriber(emitter *Emitter, worktree string) func() {
	return WireWorktreeFileSubscriber(emitter, worktree)
}

// StartLegacyEventSubscribers wires the CLI and API subscribers to emitter and
// returns the wait group that tracks those goroutines.
func StartLegacyEventSubscribers(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
	if emitter == nil {
		return nil, fmt.Errorf("emitter is nil")
	}
	if output == nil {
		output = io.Discard
	}

	wg := &sync.WaitGroup{}
	cliSubscriber := cli.NewCLISubscriber(cli.BasicWriter(output), emitter)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cliSubscriber.Start(ctx)
	}()

	if strings.TrimSpace(logsDir) != "" {
		streamSubscriber, err := stream.NewFileSubscriber(logsDir, emitter)
		if err != nil {
			fmt.Fprintf(output, "Warning: could not start stream subscriber: %v\n", err)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = streamSubscriber.Start(ctx)
			}()
		}
	}

	return wg, nil
}
