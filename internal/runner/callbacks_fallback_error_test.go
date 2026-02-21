package runner

import (
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestFallbackErrorResult_FallbackErr(t *testing.T) {
	err := fallbackErrorResult(
		"startup_error",
		"primary_err=boom",
		"provider=codex model=gpt-5",
		nil,
		errors.New("stream reset"),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "acceptance tests failed after transient fallback class=startup_error (provider=codex model=gpt-5): primary_err=boom fallback_err=stream reset"
	if got := err.Error(); got != want {
		t.Fatalf("error mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFallbackErrorResult_NilFallbackResult(t *testing.T) {
	err := fallbackErrorResult(
		"transport_disconnect",
		"primary={provider=claude model=sonnet}",
		"provider=codex model=gpt-5",
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "acceptance tests failed after transient fallback class=transport_disconnect (provider=codex model=gpt-5): primary={provider=claude model=sonnet} fallback=nil result"
	if got := err.Error(); got != want {
		t.Fatalf("error mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFallbackErrorResult_UnsuccessfulFallback(t *testing.T) {
	err := fallbackErrorResult(
		"startup_error",
		"primary={provider=claude model=sonnet}",
		"provider=codex model=gpt-5 exit_code=1 failure_category=startup_error stderr=no provider output output=no provider output diagnostics=no provider output",
		&provider.Result{Success: false},
		nil,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "acceptance tests failed after transient fallback class=startup_error: primary={provider=claude model=sonnet} fallback={provider=codex model=gpt-5 exit_code=1 failure_category=startup_error stderr=no provider output output=no provider output diagnostics=no provider output}"
	if got := err.Error(); got != want {
		t.Fatalf("error mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFallbackErrorResult_Success(t *testing.T) {
	err := fallbackErrorResult(
		"startup_error",
		"primary_err=boom",
		"provider=codex model=gpt-5",
		&provider.Result{Success: true},
		nil,
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
