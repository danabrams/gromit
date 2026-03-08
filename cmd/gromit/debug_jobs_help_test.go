package main

import (
	"strings"
	"testing"
)

func TestDebugHelpExplainsDiagnoseFixValidateWorkflow(t *testing.T) {
	t.Parallel()

	help := strings.ToLower(debugCmd.Long)
	for _, keyword := range []string{"diagnose", "fix", "validate"} {
		if !strings.Contains(help, keyword) {
			t.Fatalf("debug help should describe %q job", keyword)
		}
	}

	if !strings.Contains(help, "diagnose → fix → validate") {
		t.Fatalf("debug help should describe the diagnose → fix → validate workflow")
	}
}
