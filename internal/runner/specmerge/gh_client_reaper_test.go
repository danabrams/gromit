package specmerge

import (
	"testing"

	"github.com/danabrams/gromit/internal/procutil"
)

func TestDefaultGHReaperUsesProcessGroup(t *testing.T) {
	if defaultGHReaper != procutil.ReapProcessGroup {
		t.Fatalf("defaultGHReaper = %v, want procutil.ReapProcessGroup", defaultGHReaper)
	}
}
