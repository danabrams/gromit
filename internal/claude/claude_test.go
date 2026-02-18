package claude

import (
	"reflect"
	"sync/atomic"
	"testing"
)

func TestStartupMonitorWarnedIsAtomic(t *testing.T) {
	field, ok := reflect.TypeOf(startupMonitor{}).FieldByName("warned")
	if !ok {
		t.Fatal("expected startupMonitor.warned field to exist")
	}

	if field.Type != reflect.TypeOf(atomic.Bool{}) {
		t.Fatalf("expected startupMonitor.warned to be atomic.Bool, got %s", field.Type)
	}
}
