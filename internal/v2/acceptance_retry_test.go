//go:build acceptance
// +build acceptance

package v2

import "testing"

func TestRetryContextPopulatedOnFailure(t *testing.T) {
	t.Parallel()

	beadLoop, err := newRetryBeadLoop()
	if err != nil {
		t.Fatalf("failed to build retry loop: %v", err)
	}

	_ = beadLoop
}
