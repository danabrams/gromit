package tui

import "sync"

// fakeConversationStep describes one emission from the fake session.
type fakeConversationStep struct {
    Event      ConversationEvent
    BlockUntil <-chan struct{}
    AfterEmit  chan<- struct{}
}

// fakeConversationSession simulates a deterministic conversation.
type fakeConversationSession struct {
    steps      []fakeConversationStep
    events     chan ConversationEvent
    cancelOnce sync.Once
    cancelled  bool
    cancelCh   chan struct{}
    followUps  []string
    mu         sync.Mutex
}

func newFakeConversationSession(steps []fakeConversationStep) *fakeConversationSession {
    sess := &fakeConversationSession{
        steps:    steps,
        events:   make(chan ConversationEvent, len(steps)),
        cancelCh: make(chan struct{}),
    }
    go sess.run()
    return sess
}

func (s *fakeConversationSession) run() {
    defer close(s.events)
    for _, step := range s.steps {
        if step.BlockUntil != nil {
            <-step.BlockUntil
        }
        s.events <- step.Event
        if step.AfterEmit != nil {
            close(step.AfterEmit)
        }
    }
}

func (s *fakeConversationSession) Events() <-chan ConversationEvent {
    return s.events
}

func (s *fakeConversationSession) Cancel() {
    s.cancelOnce.Do(func() {
        s.mu.Lock()
        s.cancelled = true
        s.mu.Unlock()
        close(s.cancelCh)
    })
}

func (s *fakeConversationSession) FollowUp(prompt string) {
    s.mu.Lock()
    s.followUps = append(s.followUps, prompt)
    s.mu.Unlock()
}

func (s *fakeConversationSession) WasCancelled() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.cancelled
}

func (s *fakeConversationSession) FollowUpCalls() []string {
    s.mu.Lock()
    defer s.mu.Unlock()
    calls := make([]string, len(s.followUps))
    copy(calls, s.followUps)
    return calls
}
