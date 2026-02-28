package conversation

import "sync"

// FakeStep describes one deterministic emission from a FakeSession.
type FakeStep struct {
    Event      Event
    BlockUntil <-chan struct{}
    AfterEmit  chan<- struct{}
}

// FakeSession simulates a conversation stream for tests.
type FakeSession struct {
    steps      []FakeStep
    events     chan Event
    cancelOnce sync.Once
    cancelled  bool
    followUps  []string
    mu         sync.Mutex
}

// NewFakeSession creates a fake session that emits the provided steps in order.
func NewFakeSession(steps []FakeStep) *FakeSession {
    sess := &FakeSession{
        steps:  steps,
        events: make(chan Event, len(steps)),
    }
    go sess.run()
    return sess
}

func (s *FakeSession) run() {
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

// Events returns the channel that emits fake events.
func (s *FakeSession) Events() <-chan Event {
    return s.events
}

// Cancel marks the fake session as cancelled.
func (s *FakeSession) Cancel() {
    s.cancelOnce.Do(func() {
        s.mu.Lock()
        s.cancelled = true
        s.mu.Unlock()
    })
}

// FollowUp records follow-up prompts requested by the controller.
func (s *FakeSession) FollowUp(prompt string) {
    s.mu.Lock()
    s.followUps = append(s.followUps, prompt)
    s.mu.Unlock()
}

// WasCancelled reports whether Cancel was invoked.
func (s *FakeSession) WasCancelled() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.cancelled
}

// FollowUpCalls returns the recorded follow-up prompts.
func (s *FakeSession) FollowUpCalls() []string {
    s.mu.Lock()
    defer s.mu.Unlock()
    calls := make([]string, len(s.followUps))
    copy(calls, s.followUps)
    return calls
}
