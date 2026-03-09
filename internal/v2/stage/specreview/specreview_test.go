package specreview

import "testing"

func TestVerdictFromFindings(t *testing.T) {
    cases := []struct {
        name     string
        findings []SpecReviewFinding
        want     string
    }{
        {
            name:     "issue present",
            findings: []SpecReviewFinding{{Verdict: "issue"}},
            want:     "issue",
        },
        {
            name: "all pass",
            findings: []SpecReviewFinding{
                {Verdict: "pass"},
                {Verdict: "pass"},
            },
            want: "pass",
        },
        {
            name: "no findings",
            want: "pass",
        },
    }

    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            got := verdictFromFindings(tt.findings)
            if got != tt.want {
                t.Fatalf("%s: verdict = %q, want %q", tt.name, got, tt.want)
            }
        })
    }
}
