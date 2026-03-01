package specmerge

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/procutil"
)

func TestDefaultGHReaperUsesProcessTree(t *testing.T) {
	if reflect.ValueOf(defaultGHReaper).Pointer() != reflect.ValueOf(procutil.ReapProcessTree).Pointer() {
		t.Fatalf("defaultGHReaper should wrap procutil.ReapProcessTree")
	}
}
