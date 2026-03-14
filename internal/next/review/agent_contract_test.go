//go:build llmcontract

package review

import (
	"context"
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// RunReviewAgentContract runs the agent contract suite against any ReviewAgent implementation.
func RunReviewAgentContract(t *testing.T, agent ReviewAgent) {
	t.Run("returns parseable findings", func(t *testing.T) {
		prompt := `Review the following Go code for correctness:

func Add(a, b int) int {
    return a + b
}

func Divide(a, b int) int {
    return a / b  // potential division by zero
}

Return findings as a JSON array of objects with fields: facet, severity, file, line, description.`

		findings, err := agent.ReviewFacet(context.Background(), "correctness", prompt)
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		// Should find at least one issue (division by zero)
		if len(findings) == 0 {
			t.Error("expected at least one finding for buggy code")
		}
		for _, f := range findings {
			if f.File == "" {
				t.Error("finding missing file")
			}
			if f.Description == "" {
				t.Error("finding missing description")
			}
			if _, err := ParseSeverity(f.Severity.String()); err != nil {
				t.Errorf("finding has invalid severity: %v", f.Severity)
			}
		}
	})

	t.Run("returns empty findings for clean code", func(t *testing.T) {
		prompt := `Review the following Go code for correctness:

func Add(a, b int) int {
    return a + b
}

If no issues are found, return an empty JSON array [].
Return findings as a JSON array of objects with fields: facet, severity, file, line, description.`

		findings, err := agent.ReviewFacet(context.Background(), "correctness", prompt)
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if findings == nil {
			t.Error("expected empty slice, got nil")
		}
	})
}

func TestContract_ProviderReviewAgent(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	agent := buildRealReviewAgent(t)
	RunReviewAgentContract(t, agent)
}

func buildRealReviewAgent(t *testing.T) ReviewAgent {
	t.Helper()
	return NewProviderReviewAgent(llmadapter.ContractInvoker(t))
}
