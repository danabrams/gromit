package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
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

func TestWrapPhaseError_DeadlineExceeded_IncludesPhase(t *testing.T) {
	err := wrapPhaseError("validation", context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected wrapped error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "validation") {
		t.Fatalf("expected error to contain phase name %q, got %q", "validation", msg)
	}
	if !strings.Contains(msg, "timeout") {
		t.Fatalf("expected error to contain 'timeout', got %q", msg)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expected errors.Is(err, context.DeadlineExceeded) to be true")
	}
}
