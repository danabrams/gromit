package validator

import (
	"context"
	"testing"
)

func TestRunCheck_PassingCommand(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "echo", Command: "echo hello", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("expected pass")
	}
}

func TestRunCheck_FailingCommand(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "fail", Command: "false", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("expected fail")
	}
}
