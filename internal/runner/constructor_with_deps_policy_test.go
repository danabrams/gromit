package runner

import (
	"reflect"
	"testing"
)

func TestDepsHasPolicyFields(t *testing.T) {
	depsType := reflect.TypeOf(Deps{})
	fields := []string{"EscalationPolicy", "MethodologyPolicy", "ValidationPolicy", "StuckPolicy"}
	for _, field := range fields {
		if _, ok := depsType.FieldByName(field); !ok {
			t.Errorf("Deps missing %s field", field)
		}
	}
}
