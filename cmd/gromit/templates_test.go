package main

import (
	"strings"
	"testing"
)

func TestDefaultTemplatesGuardBead(t *testing.T) {
	templates := []struct {
		name    string
		content string
	}{
		{"build", defaultBuildTemplate},
		{"acceptance_tests", defaultAcceptanceTestsTemplate},
		{"atdd_build", defaultAtddBuildTemplate},
		{"refactor", defaultRefactorTemplate},
		{"review", defaultReviewTemplate},
		{"scope", defaultScopeTemplate},
		{"decompose", defaultDecomposeTemplate},
		{"precheck", defaultPrecheckTemplate},
		{"tdd_build", defaultTDDBuildTemplate},
	}

	for _, tc := range templates {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.content, ".Bead.") {
				t.Fatalf("template %s still references unguarded .Bead fields", tc.name)
			}
		})
	}
}
