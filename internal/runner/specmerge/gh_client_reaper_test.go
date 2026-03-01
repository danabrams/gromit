package specmerge

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/procutil"
)

func TestDefaultGHReaperUsesProcessGroup(t *testing.T) {
	if reflect.ValueOf(defaultGHReaper).Pointer() != reflect.ValueOf(procutil.ReapProcessGroup).Pointer() {
		t.Fatalf("defaultGHReaper should wrap procutil.ReapProcessGroup")
	}
}
