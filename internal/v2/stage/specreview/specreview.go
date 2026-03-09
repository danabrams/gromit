package specreview

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/jsonutil"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// SpecReviewArtifacts captures the parsed output from the spec review prompt.
type SpecReviewArtifacts struct {
	Summary  string
	Findings []SpecReviewFinding
	Verdict  string
}

// SpecReviewFinding describes a single issue or pass item produced by the review.
type SpecReviewFinding struct {
	Title         string
	Description   string
	Verdict       string
	Severity      stagepkg.SpecFindingSeverity
	Category      stagepkg.SpecFindingCategory
	Scope         stagepkg.SpecFindingScope
	AffectedFiles []string
}

// parseSpecReviewOutput reads the JSON output produced by review_spec_v2.md and
// returns the parsed artifact model.
func parseSpecReviewOutput(output string) (*SpecReviewArtifacts, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, fmt.Errorf("spec review output required")
	}

	var payload struct {
		Findings []struct {
			Verdict       string   `json:"verdict"`
			Severity      string   `json:"severity"`
			Category      string   `json:"category"`
			Scope         string   `json:"scope"`
			Description   string   `json:"description"`
			AffectedFiles []string `json:"affected_files"`
		} `json:"findings"`
		Summary string `json:"summary"`
	}

	if err := jsonutil.ExtractObject(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("parse spec review output: %w", err)
	}

	findings := make([]SpecReviewFinding, 0, len(payload.Findings))
	for _, raw := range payload.Findings {
		findings = append(findings, SpecReviewFinding{
			Title:         normalizeTitle(raw.Scope),
			Description:   strings.TrimSpace(raw.Description),
			Verdict:       strings.ToLower(strings.TrimSpace(raw.Verdict)),
			Severity:      stagepkg.SpecFindingSeverity(strings.ToLower(strings.TrimSpace(raw.Severity))),
			Category:      stagepkg.SpecFindingCategory(strings.ToLower(strings.TrimSpace(raw.Category))),
			Scope:         stagepkg.SpecFindingScope(strings.TrimSpace(raw.Scope)),
			AffectedFiles: cloneStrings(raw.AffectedFiles),
		})
	}

	return &SpecReviewArtifacts{
		Summary:  strings.TrimSpace(payload.Summary),
		Findings: findings,
		Verdict:  verdictFromFindings(findings),
	}, nil
}

func normalizeTitle(scope string) string {
	if trimmed := strings.TrimSpace(scope); trimmed != "" {
		return trimmed
	}
	return "spec review finding"
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func verdictFromFindings(findings []SpecReviewFinding) string {
	for _, finding := range findings {
		if strings.EqualFold(strings.TrimSpace(finding.Verdict), "issue") {
			return "issue"
		}
	}
	return "pass"
}

// Stage implements the spec-level review stage.
type Stage struct {
	name string
}

var _ stagepkg.Stage = (*Stage)(nil)

// New creates a spec review stage backed by the provided configuration.
func New(cfg *config.Config) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return &Stage{name: stagedesc.Describe("spec-review", cfg)}, nil
}

// Name returns the stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the spec review stage. Currently it is a no-op placeholder.
func (s *Stage) Run(_ context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}
