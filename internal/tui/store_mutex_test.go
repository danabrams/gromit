package tui

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/conversation"
)

func TestStoreHasRWMutex(t *testing.T) {
	storeType := reflect.TypeOf(Store{})
	field, ok := storeType.FieldByName("mu")
	if !ok {
		t.Fatal("Store is missing an RWMutex field named mu")
	}
	if field.Type != reflect.TypeOf(sync.RWMutex{}) {
		t.Fatalf("Store mu field is %v, want sync.RWMutex", field.Type)
	}
}

func TestStoreApplyConversationEventTransitionsStateWhileLocked(t *testing.T) {
	t.Parallel()

	store := &Store{}
	store.mu.Lock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		events := []conversation.Event{
			{Type: conversation.EventTypeStream, Text: "hello"},
			{Type: conversation.EventTypeToolWait, ToolName: "formatter"},
			{Type: conversation.EventTypeToolResult, ToolName: "formatter"},
			{Type: conversation.EventTypeDone, Text: " world"},
		}
		for _, ev := range events {
			store.ApplyConversationEvent(ev)
		}
	}()

	time.Sleep(10 * time.Millisecond)
	if store.Conversation.EventCount != 0 {
		t.Fatalf("expected event application to block while lock held, got %d", store.Conversation.EventCount)
	}

	store.mu.Unlock()
	wg.Wait()

	store.mu.RLock()
	defer store.mu.RUnlock()

	if store.Conversation.EventCount != 4 {
		t.Fatalf("EventCount=%d, want 4", store.Conversation.EventCount)
	}
	if store.Conversation.Lifecycle != ConversationLifecycleDone {
		t.Fatalf("Lifecycle=%v, want %v", store.Conversation.Lifecycle, ConversationLifecycleDone)
	}
	if len(store.Conversation.Transcript) != 1 {
		t.Fatalf("Transcript rows=%d, want 1", len(store.Conversation.Transcript))
	}
	if store.Conversation.Transcript[0].Text != "hello world" {
		t.Fatalf("Transcript[0].Text=%q, want %q", store.Conversation.Transcript[0].Text, "hello world")
	}
	if len(store.Conversation.ToolIndicators) != 2 {
		t.Fatalf("ToolIndicators=%d, want 2", len(store.Conversation.ToolIndicators))
	}
	if store.Conversation.ToolIndicators[0].Status != "waiting" {
		t.Fatalf("first tool indicator status=%q, want waiting", store.Conversation.ToolIndicators[0].Status)
	}
	if store.Conversation.ToolIndicators[1].Status != "result" {
		t.Fatalf("second tool indicator status=%q, want result", store.Conversation.ToolIndicators[1].Status)
	}
	if !store.Conversation.Session.Started {
		t.Fatal("session should be marked started")
	}
	if !store.Conversation.Session.Completed {
		t.Fatal("session should be marked completed")
	}
	if store.Conversation.Session.ToolWaitCount != 1 {
		t.Fatalf("ToolWaitCount=%d, want 1", store.Conversation.Session.ToolWaitCount)
	}
	if store.Conversation.Session.ToolResultCount != 1 {
		t.Fatalf("ToolResultCount=%d, want 1", store.Conversation.Session.ToolResultCount)
	}
	if store.Conversation.LastEvent == nil || store.Conversation.LastEvent.Type != conversation.EventTypeDone {
		t.Fatalf("LastEvent.Type=%v, want %v", store.Conversation.LastEvent, conversation.EventTypeDone)
	}
}
