package main

import (
    "strings"
    "testing"
)

func TestProfileGuidanceSnippet(t *testing.T) {
    t.Parallel()

    cases := []struct {
        name            string
        profile         string
        wantContains    []string
        wantNotContains []string
    }{
        {
            name:            "go profile",
            profile:         "go",
            wantContains:    []string{"go test", "go build", "go vet"},
        },
        {
            name:            "node profile",
            profile:         "node",
            wantContains:    []string{"npm test", "npm run build"},
            wantNotContains: []string{"go test"},
        },
        {
            name:            "python profile",
            profile:         "python",
            wantContains:    []string{"pytest"},
            wantNotContains: []string{"go test"},
        },
        {
            name:            "custom profile",
            profile:         "custom",
            wantContains:    []string{"customize"},
            wantNotContains: []string{"go test"},
        },
    }

    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()

            snippet := profileGuidanceSnippet(tc.profile)
            if snippet == "" {
                t.Fatalf("profileGuidanceSnippet returned empty string for %q profile", tc.profile)
            }

            for _, want := range tc.wantContains {
                if !strings.Contains(snippet, want) {
                    t.Errorf("snippet for %q profile missing %q, got %q", tc.profile, want, snippet)
                }
            }

            for _, not := range tc.wantNotContains {
                if strings.Contains(snippet, not) {
                    t.Errorf("snippet for %q profile unexpectedly contained %q", tc.profile, not)
                }
            }
        })
    }
}
