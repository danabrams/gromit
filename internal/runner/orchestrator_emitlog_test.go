package runner

import (
	"bytes"
	"testing"

	"github.com/danabrams/gromit/internal/events"
)

func TestEmitLogWritesToOutputWhenNoSubscribers(t *testing.T) {
	var output bytes.Buffer

	o := &Orchestrator{
		cfg:     OrchestratorConfig{Output: &output},
		emitter: events.NewEmitter(),
	}

	o.logWarning("early warning: %s", "failure")

	got := output.String()
	want := "[warning] early warning: failure\n"
	if got != want {
		t.Fatalf("unexpected output; got %q want %q", got, want)
	}
}
