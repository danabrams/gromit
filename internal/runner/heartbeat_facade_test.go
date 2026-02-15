package runner

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestOverwriteHeartbeat_LogsWriteError(t *testing.T) {
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("NewStreamStats() failed: %v", err)
	}

	buf := &bytes.Buffer{}
	r := &Runner{
		output:  buf,
		syncOut: newSyncWriter(errWriter{}),
	}

	r.overwriteHeartbeat(stats, "previous")

	if !strings.Contains(buf.String(), "Warning: failed to overwrite heartbeat: write failed") {
		t.Fatalf("expected warning in output, got %q", buf.String())
	}
}
