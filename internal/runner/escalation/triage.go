package escalation

import (
	"regexp"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// FailureLayer is the top-level failure taxonomy bucket.
type FailureLayer string

const (
	LayerProviderTransport FailureLayer = "provider_transport"
	LayerEnvironment       FailureLayer = "environment"
	LayerOrchestration     FailureLayer = "orchestration"
	LayerCode              FailureLayer = "code"
)

// TriageResult captures a normalized failure classification.
type TriageResult struct {
	Layer       FailureLayer
	SubCategory string
	Detail      string
	Retryable   bool
}

var (
	missingToolPattern       = regexp.MustCompile(`exec: .+: executable file not found`)
	goVersionMismatchPattern = regexp.MustCompile(`go: go\.mod requires go >=`)
	noSpacePattern           = regexp.MustCompile(`no space left on device`)
	permissionDeniedPattern  = regexp.MustCompile(`permission denied`)
)

// Triage classifies invocation failures using a deterministic waterfall.
func Triage(inv *runtypes.InvocationResult, bc *runtypes.BeadContext) *TriageResult {
	if inv != nil && inv.ProviderResult != nil {
		switch inv.ProviderResult.FailureCategory {
		case provider.FailureCategoryTransportDisconnect:
			return &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "disconnect",
				Detail:      provider.FailureCategoryTransportDisconnect,
				Retryable:   true,
			}
		case provider.FailureCategoryRateLimited:
			return &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "rate_limit",
				Detail:      provider.FailureCategoryRateLimited,
				Retryable:   true,
			}
		case provider.FailureCategoryAuth:
			return &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "auth",
				Detail:      provider.FailureCategoryAuth,
				Retryable:   false,
			}
		}

		detail := inv.ProviderResult.Stderr
		if detail == "" {
			detail = inv.ProviderResult.Output
		}
		switch {
		case missingToolPattern.MatchString(detail):
			return &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "missing_tool",
				Detail:      detail,
				Retryable:   false,
			}
		case goVersionMismatchPattern.MatchString(detail):
			return &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "version_mismatch",
				Detail:      detail,
				Retryable:   false,
			}
		case noSpacePattern.MatchString(detail):
			return &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "resource_exhausted",
				Detail:      detail,
				Retryable:   false,
			}
		case permissionDeniedPattern.MatchString(detail):
			return &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "permission",
				Detail:      detail,
				Retryable:   false,
			}
		}
	}

	if bc != nil {
		if bc.BuildPrompt == "" {
			return &TriageResult{
				Layer:       LayerOrchestration,
				SubCategory: "bad_prompt",
				Detail:      "build prompt is empty",
				Retryable:   false,
			}
		}
		if bc.Bead != nil && bc.Bead.Description == "" {
			return &TriageResult{
				Layer:       LayerOrchestration,
				SubCategory: "bad_bead",
				Detail:      "bead description is empty",
				Retryable:   false,
			}
		}
	}

	return &TriageResult{
		Layer:       LayerCode,
		SubCategory: "default",
		Detail:      "code-level or unknown failure",
		Retryable:   true,
	}
}
