package validate

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestNewAutoFixFn_FormatsGoFiles(t *testing.T) {
	var calls []string
	runner := func(ctx context.Context, name string, args ...string) (string, error) {
		call := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
		calls = append(calls, call)
		if name == "git" {
			return "a.go\nb.txt\nc.go\na.go\n", nil
		}
		return "", nil
	}

	autoFix := NewAutoFixFn(runner)
	if autoFix == nil {
		t.Fatalf("NewAutoFixFn returned nil")
	}
	if err := autoFix("start-commit"); err != nil {
		t.Fatalf("AutoFixFn error = %v", err)
	}

	want := []string{
		"git diff --name-only start-commit",
		"gofmt -w a.go",
		"goimports -w a.go",
		"gofmt -w c.go",
		"goimports -w c.go",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("runner calls = %v, want %v", calls, want)
	}
}
