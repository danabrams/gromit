package escalation

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrSameScopeRetryBlocked_PreservesMessageInChain(t *testing.T) {
	err := fmt.Errorf("same-scope retry flow: %w", ErrSameScopeRetryBlocked)
	if !errors.Is(err, ErrSameScopeRetryBlocked) {
		t.Fatalf("errors.Is should detect ErrSameScopeRetryBlocked, got %v", err)
	}
	if !strings.Contains(err.Error(), sameScopeRetryBlockedMessage) {
		t.Fatalf("error chain should include block message, got %q", err.Error())
	}
}

func TestErrPartialDecompositionState_IsDetectable(t *testing.T) {
	err := fmt.Errorf("decomposition state alert: %w", ErrPartialDecompositionState)
	if !errors.Is(err, ErrPartialDecompositionState) {
		t.Fatalf("errors.Is should detect ErrPartialDecompositionState, got %v", err)
	}
	if !strings.Contains(err.Error(), partialDecompositionStateMessage) {
		t.Fatalf("error string should mention partial decomposition state, got %q", err.Error())
	}
}
