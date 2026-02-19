package runner

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/runner/policy"
)

func TestRunnerHasPolicyFields(t *testing.T) {
	runnerType := reflect.TypeOf(Runner{})
	tests := []struct {
		field    string
		wantType reflect.Type
	}{
		{"stuckPolicy", reflect.TypeOf((*policy.StuckPolicy)(nil)).Elem()},
		{"escalationPolicy", reflect.TypeOf((*policy.EscalationPolicy)(nil)).Elem()},
		{"validationPolicy", reflect.TypeOf((*policy.ValidationPolicy)(nil)).Elem()},
		{"methodologyPolicy", reflect.TypeOf((*policy.MethodologyPolicy)(nil)).Elem()},
	}
	for _, tt := range tests {
		f, ok := runnerType.FieldByName(tt.field)
		if !ok {
			t.Errorf("Runner missing field %s", tt.field)
			continue
		}
		if f.Type != tt.wantType {
			t.Errorf("Runner.%s has type %v, want %v", tt.field, f.Type, tt.wantType)
		}
	}
}
