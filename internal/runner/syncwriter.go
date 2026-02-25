package runner

import (
	"io"
	"sync"

	"github.com/danabrams/gromit/internal/runner/execution"
)

// syncWriter wraps an io.Writer with thread-safe writes and automatic newline
// handling for transitions between overwrite mode (heartbeat updates) and
// normal writes (streaming output).
type syncWriter struct {
	w                io.Writer
	mu               sync.Mutex
	lastWasOverwrite bool
}

// newSyncWriter creates a new synchronized writer that wraps the given io.Writer.
func newSyncWriter(w io.Writer) *syncWriter {
	return &syncWriter{w: w}
}

// Write implements io.Writer. It serializes writes with a mutex and automatically
// prepends a newline if the previous write was an overwrite (to ensure text
// output starts on a fresh line after in-place heartbeat updates).
func (sw *syncWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// If the last write was an overwrite, prepend a newline to start fresh
	if sw.lastWasOverwrite {
		if _, err := sw.w.Write([]byte("\n")); err != nil {
			return 0, err
		}
		sw.lastWasOverwrite = false
	}

	return sw.w.Write(p)
}

// WriteOverwrite writes data without trailing newline and marks the write as
// an overwrite. This is used for in-place terminal updates like heartbeat status.
// The next call to Write() will automatically prepend a newline to ensure proper
// separation from subsequent output.
func (sw *syncWriter) WriteOverwrite(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	n, err = sw.w.Write(p)
	if err == nil {
		sw.lastWasOverwrite = true
	}
	return n, err
}

// Compile-time interface check
var _ execution.OverwriteWriter = (*syncWriter)(nil)
