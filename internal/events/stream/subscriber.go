package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

// Subscriber consumes events and writes them to a newline-delimited structured stream.
type Subscriber interface {
	Start(ctx context.Context) error
	// Path returns the path to the underlying stream file, if known.
	Path() string
}

type writerSubscriber struct {
	emitter *events.Emitter
	writer  io.WriteCloser
	path    string
}

// NewWriterSubscriber attaches a writer-backed subscriber to the emitter.
func NewWriterSubscriber(writer io.WriteCloser, emitter *events.Emitter) Subscriber {
	return &writerSubscriber{
		emitter: emitter,
		writer:  writer,
	}
}

// NewFileSubscriber creates a structured event stream file in logsDir and wires it to the emitter.
func NewFileSubscriber(logsDir string, emitter *events.Emitter) (Subscriber, error) {
	if logsDir == "" {
		return nil, fmt.Errorf("logsDir is empty")
	}
	if emitter == nil {
		return nil, fmt.Errorf("emitter is nil")
	}

	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating logs directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := filepath.Join(logsDir, fmt.Sprintf("events-%s.jsonl", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("creating structured event stream file: %w", err)
	}

	return &writerSubscriber{
		emitter: emitter,
		writer:  file,
		path:    filename,
	}, nil
}

func (s *writerSubscriber) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("subscriber is nil")
	}
	if s.emitter == nil {
		return fmt.Errorf("emitter is nil")
	}
	if s.writer == nil {
		return fmt.Errorf("writer is nil")
	}

	ch := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(ch)
	defer s.writer.Close()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := s.writeEvent(event); err != nil {
				return fmt.Errorf("serialize event: %w", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *writerSubscriber) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *writerSubscriber) writeEvent(event events.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	line := structuredEvent{
		Type:    event.EventType(),
		Time:    event.EventTime(),
		Payload: json.RawMessage(payload),
	}

	encoded, err := json.Marshal(line)
	if err != nil {
		return err
	}

	encoded = append(encoded, '\n')
	if _, err := s.writer.Write(encoded); err != nil {
		return err
	}

	return nil
}

type structuredEvent struct {
	Type    string          `json:"type"`
	Time    time.Time       `json:"time"`
	Payload json.RawMessage `json:"payload"`
}

var _ Subscriber = (*writerSubscriber)(nil)
