package main

import (
	"strings"
	"testing"
)

func TestDebugHelpExplainsDebugJobsWorkflow(t *testing.T) {
	t.Parallel()

	help := strings.ToLower(debugCmd.Long)
	for _, keyword := range []string{"diagnose", "fix", "present recommendations"} {
		if !strings.Contains(help, keyword) {
			t.Fatalf("debug help should describe %q job", keyword)
		}
	}

	if !strings.Contains(help, "diagnose → fix → present recommendations") {
		t.Fatalf("debug help should describe the diagnose → fix → present recommendations workflow")
	}
}
