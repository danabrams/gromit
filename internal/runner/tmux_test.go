package runner

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRestoreTmuxTitle_LogsError(t *testing.T) {
	buf := &bytes.Buffer{}
	r := &Runner{
		output: buf,
	}

	r.restoreTmuxTitle(func() error {
		return errors.New("restore failed")
	})

	if !strings.Contains(buf.String(), "Warning: failed to restore tmux title: restore failed") {
		t.Fatalf("expected warning in output, got %q", buf.String())
	}
}
