package event

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type eventCorrelation struct {
	BeadID    string `json:"bead_id,omitempty"`
	StageName string `json:"stage_name,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
}

// FileSubscriber appends JSON-encoded events to a JSONL file.
type FileSubscriber struct {
	path        string
	mu          sync.Mutex
	file        *os.File
	beadContext map[string]eventCorrelation
}

// NewFileSubscriber returns a FileSubscriber that writes to path.
func NewFileSubscriber(path string) *FileSubscriber {
	return &FileSubscriber{
		path:        path,
		beadContext: make(map[string]eventCorrelation),
	}
}

// NewWorktreeFileSubscriber returns a FileSubscriber that writes to
// <worktree>/.gromit/v2/events.jsonl.
func NewWorktreeFileSubscriber(worktree string) *FileSubscriber {
	worktree = strings.TrimSpace(worktree)
	if worktree == "" {
		worktree = "."
	}
	return NewFileSubscriber(filepath.Join(worktree, ".gromit", "v2", "events.jsonl"))
}

// SubscribeTo registers Handle as a subscriber on emitter and returns
// an unsubscribe callback.
func (f *FileSubscriber) SubscribeTo(emitter *Emitter) func() {
	if emitter == nil {
		return func() {}
	}
	return emitter.Subscribe(f.Handle)
}

// Handle appends the event as a JSON line to the JSONL file.
// The file is opened lazily on the first call and held open for subsequent calls.
// Parent directories are created automatically on first call.
func (f *FileSubscriber) Handle(evt TypedEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := f.marshalWithCorrelation(evt)
	if err != nil {
		log.Printf("file subscriber: marshal %s: %v", f.path, err)
		return
	}
	data = append(data, '\n')

	if f.file == nil {
		if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
			log.Printf("file subscriber: mkdir %s: %v", f.path, err)
			return
		}
		file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("file subscriber: open %s: %v", f.path, err)
			return
		}
		f.file = file
	}
	if _, err := f.file.Write(data); err != nil {
		log.Printf("file subscriber: write %s: %v", f.path, err)
	}
}

func (f *FileSubscriber) marshalWithCorrelation(evt TypedEvent) ([]byte, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return nil, err
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return data, nil
	}

	correlation := f.deriveCorrelation(payload)
	if !correlation.empty() {
		correlationJSON, err := json.Marshal(correlation)
		if err != nil {
			return nil, err
		}
		payload["correlation"] = correlationJSON
	}

	return json.Marshal(payload)
}

func (f *FileSubscriber) deriveCorrelation(payload map[string]json.RawMessage) eventCorrelation {
	beadID := readStringField(payload, "bead_id")
	stageName := readStringField(payload, "stage_name")
	iteration := readIntField(payload, "iteration")

	if beadID == "" && stageName == "" && iteration == 0 {
		return eventCorrelation{}
	}

	if beadID != "" {
		prior := f.beadContext[beadID]
		if stageName == "" {
			stageName = prior.StageName
		}
		if iteration == 0 {
			iteration = prior.Iteration
		}

		if prior.BeadID == "" {
			prior.BeadID = beadID
		}
		if stageName != "" {
			prior.StageName = stageName
		}
		if iteration > 0 {
			prior.Iteration = iteration
		}
		f.beadContext[beadID] = prior
	}

	return eventCorrelation{
		BeadID:    beadID,
		StageName: stageName,
		Iteration: iteration,
	}
}

func readStringField(payload map[string]json.RawMessage, key string) string {
	raw, ok := payload[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func readIntField(payload map[string]json.RawMessage, key string) int {
	raw, ok := payload[key]
	if !ok {
		return 0
	}
	var value int
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}

	var numericValue float64
	if err := json.Unmarshal(raw, &numericValue); err == nil {
		return int(numericValue)
	}
	return 0
}

func (c eventCorrelation) empty() bool {
	return c.BeadID == "" && c.StageName == "" && c.Iteration == 0
}

// Close flushes and closes the underlying file handle. It is safe to call
// multiple times; subsequent calls are no-ops.
func (f *FileSubscriber) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}
