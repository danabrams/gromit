//go:build acceptance
// +build acceptance

package v2

import (
	"os/exec"
	"testing"
)

func TestAcceptanceFilesGofmtCompliant(t *testing.T) {
	t.Helper()

	files := []string{
		"acceptance_test.go",
		"event/event.go",
	}

	cmd := exec.Command("gofmt", append([]string{"-l"}, files...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running gofmt: %v\n%s", err, output)
	}
	if out := string(output); out != "" {
		t.Fatalf("acceptance files not gofmt compliant:\n%s", out)
	}
}
