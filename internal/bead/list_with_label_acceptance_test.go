//go:build acceptance

package bead

import (
	"fmt"
	"testing"
)

// TestListWithLabel_ReturnsUnlimitedResults keeps one high-value acceptance
// check that proves ListWithLabel uses --limit 0 by validating >50 results.
func TestListWithLabel_ReturnsUnlimitedResults(t *testing.T) {
	c := newIsolatedClient(t)

	const beadCount = 51
	const testLabel = "spec:unlimited-test"
	for i := 0; i < beadCount; i++ {
		_, err := c.Create(fmt.Sprintf("Unlimited bead %02d", i), 1, []string{testLabel}, []string{})
		if err != nil {
			t.Fatalf("Cannot create test bead %d: %v", i, err)
		}
	}

	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}
	if len(beads) != beadCount {
		t.Fatalf("ListWithLabel(%q) returned %d beads, expected %d", testLabel, len(beads), beadCount)
	}
}
