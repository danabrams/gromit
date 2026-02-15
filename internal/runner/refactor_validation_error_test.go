package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWrapRefactorValidationError(t *testing.T) {
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
			name:   "other validation failure",
			err:    errors.New("validation failed"),
			want:   "validation failed after refactoring",
			wantIs: nil,
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
