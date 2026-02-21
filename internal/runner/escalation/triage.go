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
		if result := triageProviderTransport(inv.ProviderResult); result != nil {
			return result
		}
		if result := triageEnvironment(inv.ProviderResult); result != nil {
			return result
		}
	}

	if result := triageOrchestration(bc); result != nil {
		return result
	}

	return triageResult(LayerCode, "default", "code-level or unknown failure", true)
}

func triageProviderTransport(result *provider.Result) *TriageResult {
	switch result.FailureCategory {
	case provider.FailureCategoryTransportDisconnect:
		return triageResult(LayerProviderTransport, "disconnect", provider.FailureCategoryTransportDisconnect, true)
	case provider.FailureCategoryRateLimited:
		return triageResult(LayerProviderTransport, "rate_limit", provider.FailureCategoryRateLimited, true)
	case provider.FailureCategoryAuth:
		return triageResult(LayerProviderTransport, "auth", provider.FailureCategoryAuth, false)
	default:
		return nil
	}
}

func triageEnvironment(result *provider.Result) *TriageResult {
	detail := result.Stderr
	if detail == "" {
		detail = result.Output
	}

	switch {
	case missingToolPattern.MatchString(detail):
		return triageResult(LayerEnvironment, "missing_tool", detail, false)
	case goVersionMismatchPattern.MatchString(detail):
		return triageResult(LayerEnvironment, "version_mismatch", detail, false)
	case noSpacePattern.MatchString(detail):
		return triageResult(LayerEnvironment, "resource_exhausted", detail, false)
	case permissionDeniedPattern.MatchString(detail):
		return triageResult(LayerEnvironment, "permission", detail, false)
	default:
		return nil
	}
}

func triageOrchestration(bc *runtypes.BeadContext) *TriageResult {
	if bc == nil {
		return nil
	}
	if bc.BuildPrompt == "" {
		return triageResult(LayerOrchestration, "bad_prompt", "build prompt is empty", false)
	}
	if bc.Bead != nil && bc.Bead.Description == "" {
		return triageResult(LayerOrchestration, "bad_bead", "bead description is empty", false)
	}
	return nil
}

func triageResult(layer FailureLayer, subCategory, detail string, retryable bool) *TriageResult {
	return &TriageResult{
		Layer:       layer,
		SubCategory: subCategory,
		Detail:      detail,
		Retryable:   retryable,
	}
}
