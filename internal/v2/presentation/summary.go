package presentation

import (
	"strings"

	"github.com/danabrams/gromit/internal/config"
	v2review "github.com/danabrams/gromit/internal/v2/review"
)

const specBranchPrefix = "gromit/spec/"

// AcceptanceResult captures a single acceptance check outcome.
type AcceptanceResult struct {
	Title       string
	Description string
}

// BeadSummary describes the work completed by a bead invocation.
type BeadSummary struct {
	ID          string
	Title       string
	Description string
}

// PresentationSummary captures the data required to render a presentation to a product owner.
type PresentationSummary struct {
	SpecName           string
	SpecBranch         string
	IntegrationBranch  string
	Plan               string
	Worktree           string
	BeadSummaries      []BeadSummary
	Success            bool
	AcceptanceResults  []AcceptanceResult
	OutOfScopeFindings []v2review.Finding
	FailureSummary     string
	RemainingWork      []string
	BranchLink         string
	DiffLink           string
}

// SpecBranchName returns the canonical spec branch for the provided spec.
func SpecBranchName(specName string) string {
	trimmed := strings.TrimSpace(specName)
	if trimmed == "" {
		return ""
	}
	return specBranchPrefix + trimmed
}

// DefaultIntegrationBranch returns the branch that should receive spec worktree merges by default.
func DefaultIntegrationBranch() string {
	return config.DefaultBaseBranch
}

// RenderPRBody renders the body of the GitHub pull request for the provided summary.
func RenderPRBody(summary PresentationSummary) string {
	if summary.Success {
		return renderSuccessBody(summary)
	}
	return renderFailureBody(summary)
}

func renderSuccessBody(summary PresentationSummary) string {
	var b strings.Builder
	b.WriteString("## Acceptance Results\n")
	if len(summary.AcceptanceResults) == 0 {
		b.WriteString("- Not provided\n")
	} else {
		for _, result := range summary.AcceptanceResults {
			b.WriteString("- " + strings.TrimSpace(result.Title))
			if desc := strings.TrimSpace(result.Description); desc != "" {
				b.WriteString(": " + desc)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Out-of-Scope Findings\n")
	if len(summary.OutOfScopeFindings) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, finding := range summary.OutOfScopeFindings {
			b.WriteString("- " + strings.TrimSpace(finding.Title))
			if desc := strings.TrimSpace(finding.Description); desc != "" {
				b.WriteString(": " + desc)
			}
			b.WriteString("\n")
			if len(finding.AffectedFiles) > 0 {
				b.WriteString("  Affected files: " + strings.Join(finding.AffectedFiles, ", ") + "\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func renderFailureBody(summary PresentationSummary) string {
	var b strings.Builder
	b.WriteString("## Failure Summary\n")
	if spec := strings.TrimSpace(summary.SpecName); spec != "" {
		b.WriteString("Spec: " + spec + "\n")
	}
	failure := strings.TrimSpace(summary.FailureSummary)
	if failure == "" {
		b.WriteString("- Not available\n")
	} else {
		b.WriteString(failure + "\n")
	}
	b.WriteString("\n## Remaining Work\n")
	remainingWritten := 0
	for _, work := range summary.RemainingWork {
		cleaned := strings.TrimSpace(work)
		if cleaned == "" {
			continue
		}
		b.WriteString("- " + cleaned + "\n")
		remainingWritten++
	}
	if remainingWritten == 0 {
		b.WriteString("- None\n")
	}
	return strings.TrimSpace(b.String())
}
