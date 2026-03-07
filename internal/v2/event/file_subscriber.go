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
	path    string
	mu      sync.Mutex
	mkdirOnce sync.Once
	mkdirErr  error
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
// Parent directories are created automatically.
func (f *FileSubscriber) Handle(evt TypedEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		log.Printf("file subscriber: marshal %s: %v", f.path, err)
		return
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

	f.mkdirOnce.Do(func() {
		f.mkdirErr = os.MkdirAll(filepath.Dir(f.path), 0o755)
	})
	if f.mkdirErr != nil {
		log.Printf("file subscriber: mkdir %s: %v", f.path, f.mkdirErr)
		return
	}
	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("file subscriber: open %s: %v", f.path, err)
		return
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		log.Printf("file subscriber: write %s: %v", f.path, err)
	}
}
