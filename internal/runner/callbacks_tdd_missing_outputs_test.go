package runner

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestWarnTddFreshContextMissingOutputs_LogsWhenNoRequirements(t *testing.T) {
	t.Parallel()
	buf := new(bytes.Buffer)
	warnTddFreshContextMissingOutputs(buf, &bead.Bead{ID: "bead-1", Title: "  "})

	if buf.Len() == 0 {
		t.Fatalf("expected warning to be emitted when no requirements are available")
	}
	if !strings.Contains(buf.String(), "bead-1") {
		t.Fatalf("warning should reference the bead ID; got %q", buf.String())
	}
}

func TestWarnTddFreshContextMissingOutputs_SkipsWhenTitlePresent(t *testing.T) {
	t.Parallel()
	buf := new(bytes.Buffer)
	warnTddFreshContextMissingOutputs(buf, &bead.Bead{ID: "bead-2", Title: "Implement feature"})

	if buf.Len() != 0 {
		t.Fatalf("expected no warning when requirements exist, got %q", buf.String())
	}
}
