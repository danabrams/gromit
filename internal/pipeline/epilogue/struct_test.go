package epilogue

import (
	"reflect"
	"testing"
)

func TestEpilogueStructDoesNotDeclareDeprecatedDependencies(t *testing.T) {
	typ := reflect.TypeOf(Epilogue{})
	deprecatedFields := []string{"worktree", "branchRemover", "specgate"}
	for _, field := range deprecatedFields {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("Epilogue struct should not declare %q (deprecated dependency)", field)
		}
	}
}
