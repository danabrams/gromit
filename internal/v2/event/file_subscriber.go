package event

import "sync"

// FileSubscriber appends JSON-encoded events to a JSONL file.
type FileSubscriber struct {
	path string
	mu   sync.Mutex
}

// NewFileSubscriber returns a FileSubscriber that writes to path.
func NewFileSubscriber(path string) *FileSubscriber {
	return &FileSubscriber{path: path}
}
