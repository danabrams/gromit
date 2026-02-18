package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestWrapRefactorValidationError(t *testing.T) {
	genericErr := errors.New("generic validation failure")

	tests := []struct {
		name   string
		err    error
		want   string
		wantIs error
	}{
		{
			name:   "deadline exceeded",
			err:    context.DeadlineExceeded,
			want:   "validation after refactoring aborted due to timeout budget exhaustion",
			wantIs: context.DeadlineExceeded,
		},
		{
			name:   "canceled",
			err:    context.Canceled,
			want:   "validation after refactoring aborted",
			wantIs: context.Canceled,
		},
		{
			name:   "generic validation error wraps original",
			err:    genericErr,
			want:   "validation failed after refactoring",
			wantIs: genericErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapRefactorValidationError(tc.err)
			if got == nil {
				t.Fatal("expected wrapped error, got nil")
			}
			if !strings.Contains(got.Error(), tc.want) {
				t.Fatalf("expected wrapped error to contain %q, got %q", tc.want, got.Error())
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("expected errors.Is(..., %v) to be true, got %v", tc.wantIs, got)
			}
		})
	}
}

func TestSetPhaseAttribution_SetsTimeoutPhaseOnDeadlineExceeded(t *testing.T) {
	result := &runtypes.IterationResult{}
	setPhaseAttribution(result, "validation", context.DeadlineExceeded)
	if result.TimeoutPhase != "validation" {
		t.Fatalf("TimeoutPhase = %q, want %q", result.TimeoutPhase, "validation")
	}
}

func TestSetPhaseAttribution_SetsTimeoutPhaseOnCanceled(t *testing.T) {
	result := &runtypes.IterationResult{}
	setPhaseAttribution(result, "refactor", context.Canceled)
	if result.TimeoutPhase != "refactor" {
		t.Fatalf("TimeoutPhase = %q, want %q", result.TimeoutPhase, "refactor")
	}
}

func TestSetPhaseAttribution_NoOpForGenericErrors(t *testing.T) {
	result := &runtypes.IterationResult{}
	setPhaseAttribution(result, "red", errors.New("some error"))
	if result.TimeoutPhase != "" {
		t.Fatalf("TimeoutPhase = %q, want empty for generic errors", result.TimeoutPhase)
	}
}

func TestSetPhaseAttribution_NoOpForNilError(t *testing.T) {
	result := &runtypes.IterationResult{}
	setPhaseAttribution(result, "green", nil)
	if result.TimeoutPhase != "" {
		t.Fatalf("TimeoutPhase = %q, want empty for nil error", result.TimeoutPhase)
	}
}

func TestWrapPhaseError(t *testing.T) {
	genericErr := errors.New("something broke")

	tests := []struct {
		name      string
		phase     string
		err       error
		wantPhase string
		wantKind  string
		wantIs    error
	}{
		{
			name:      "deadline exceeded includes phase and timeout",
			phase:     "validation",
			err:       context.DeadlineExceeded,
			wantPhase: "validation",
			wantKind:  "timeout",
			wantIs:    context.DeadlineExceeded,
		},
		{
			name:      "canceled includes phase and canceled",
			phase:     "red",
			err:       context.Canceled,
			wantPhase: "red",
			wantKind:  "canceled",
			wantIs:    context.Canceled,
		},
		{
			name:      "generic error includes phase and failed",
			phase:     "refactor",
			err:       genericErr,
			wantPhase: "refactor",
			wantKind:  "failed",
			wantIs:    genericErr,
		},
		{
			name:      "nil error returns nil",
			phase:     "green",
			err:       nil,
			wantPhase: "",
			wantKind:  "",
			wantIs:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapPhaseError(tc.phase, tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected wrapped error, got nil")
			}
			msg := got.Error()
			if !strings.Contains(msg, tc.wantPhase) {
				t.Fatalf("expected error to contain phase %q, got %q", tc.wantPhase, msg)
			}
			if !strings.Contains(msg, tc.wantKind) {
				t.Fatalf("expected error to contain %q, got %q", tc.wantKind, msg)
			}
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Fatalf("expected errors.Is(..., %v) to be true", tc.wantIs)
			}
		})
	}
}
