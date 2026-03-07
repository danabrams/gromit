package event

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// FileSubscriber appends JSON-encoded events to a JSONL file.
type FileSubscriber struct {
	path string
	mu   sync.Mutex
	file *os.File
}

// NewFileSubscriber returns a FileSubscriber that writes to path.
func NewFileSubscriber(path string) *FileSubscriber {
	return &FileSubscriber{path: path}
}

// SubscribeTo registers Handle as a subscriber on emitter.
func (f *FileSubscriber) SubscribeTo(emitter *Emitter) {
	emitter.Subscribe(f.Handle)
}

// Handle appends the event as a JSON line to the JSONL file.
// The file is opened lazily on the first call and held open for subsequent calls.
// Parent directories are created automatically on first call.
func (f *FileSubscriber) Handle(evt TypedEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		log.Printf("file subscriber: marshal %s: %v", f.path, err)
		return
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

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
