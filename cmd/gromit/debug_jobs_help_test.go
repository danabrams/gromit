package main

import (
	"strings"
	"testing"
)

func TestDebugHelpDescribesDiagnoseFixLearnJobs(t *testing.T) {
	t.Parallel()

	help := strings.ToLower(debugCmd.Long)
	for _, keyword := range []string{"diagnose", "fix", "learn"} {
		if !strings.Contains(help, keyword) {
			t.Fatalf("debug help should describe %q job", keyword)
		}
	}
}
