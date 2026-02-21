//go:build integration && !acceptance

package bead

import (
	"os/exec"
	"testing"
)

// newIsolatedClient creates a bd client that operates in a temp directory
// so tests don't pollute the real project's beads database.
func newIsolatedClient(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("bd init not available: %v: %s", err, out)
	}
	return &Client{binary: "bd", Dir: dir}
}
