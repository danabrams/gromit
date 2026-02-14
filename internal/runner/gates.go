package runner

import (
	"context"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// runPrecheck calls configured model with precheck prompt to check if acceptance criteria are already met.
// Returns true if precheck passed (criteria already satisfied), and the duration it took.
// When verification is enabled (default), a second check with a medium-tier model must also agree.
// Non-blocking: logs warnings on errors and returns false.
func (r *Runner) runPrecheck(ctx context.Context, b *bead.Bead) (bool, time.Duration) {
	start := time.Now()

	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.router == nil {
		return false, 0
	}

	if !r.cfg.Precheck.IsEnabled() {
		return false, 0
	}

	// Deterministic file existence check — reject before invoking any model
	// if the bead describes creating files that don't exist yet. Models
	// (especially Codex) unreliably verify file existence for refactoring beads
	// where build/test criteria pass both before and after the change.
	if parsed := extractExpectedFiles(b.Description); len(parsed) > 0 && anyFileMissing(parsed) {
		r.log("Pre-check: description mentions files to create that don't exist, skipping model check")
		return false, time.Since(start)
	}

	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead for precheck: %v", err)
	}

	precheckCtx := &prompt.PrecheckContext{
		Bead:       b,
		ParentBead: parent,
	}

	precheckPrompt, err := r.renderer.RenderPrecheck(precheckCtx)
	if err != nil {
		r.log("Warning: failed to render precheck prompt: %v", err)
		return false, time.Since(start)
	}

	// Phase 1: Screen with low tier (haiku)
	precheckTimeout := time.Duration(r.cfg.Precheck.TimeoutSeconds) * time.Second
	precheckCtx2, cancel := context.WithTimeout(ctx, precheckTimeout)
	defer cancel()

	p, _ := r.router.Select("precheck", provider.TierLow)
	if p == nil {
		r.log("Warning: no provider available for precheck")
		return false, time.Since(start)
	}

	result, err := p.Run(precheckCtx2, precheckPrompt, provider.TierLow)
	if err != nil {
		r.log("Warning: precheck invocation failed: %v", err)
		return false, time.Since(start)
	}
	if result == nil {
		r.log("Warning: precheck returned nil result")
		return false, time.Since(start)
	}

	if !result.Success {
		r.log("Warning: precheck failed with exit code %d", result.ExitCode)
		return false, time.Since(start)
	}

	passed := strings.Contains(result.Output, "PRECHECK_PASSED")

	if !passed {
		r.log("Pre-check: acceptance criteria not yet met")
		return false, time.Since(start)
	}

	// Phase 1 passed — run verification if enabled
	if !r.cfg.Precheck.Verification.IsVerificationEnabled() {
		r.log("Pre-check: acceptance criteria already met")
		return true, time.Since(start)
	}

	// Phase 2: Verify with medium tier (sonnet)
	r.log("Pre-check: phase 1 passed, running verification")
	verifyTimeout := time.Duration(r.cfg.Precheck.Verification.TimeoutSeconds) * time.Second
	verifyCtx, verifyCancel := context.WithTimeout(ctx, verifyTimeout)
	defer verifyCancel()

	vp, _ := r.router.Select("precheck", provider.TierMedium)
	if vp == nil {
		r.log("Warning: no provider available for precheck verification, proceeding to build")
		return false, time.Since(start)
	}

	verifyResult, err := vp.Run(verifyCtx, precheckPrompt, provider.TierMedium)
	if err != nil {
		r.log("Warning: precheck verification failed: %v, proceeding to build", err)
		return false, time.Since(start)
	}
	if verifyResult == nil {
		r.log("Warning: precheck verification returned nil result, proceeding to build")
		return false, time.Since(start)
	}

	if !verifyResult.Success {
		r.log("Warning: precheck verification exited with code %d, proceeding to build", verifyResult.ExitCode)
		return false, time.Since(start)
	}

	verified := strings.Contains(verifyResult.Output, "PRECHECK_PASSED")

	if verified {
		r.log("Pre-check: acceptance criteria already met (verified)")
	} else {
		r.log("Pre-check: phase 1 passed but verification rejected, proceeding to build")
	}

	return verified, time.Since(start)
}

// checkScope calls haiku with scope prompt and returns ScopeEstimate.
// If scope check fails, logs a warning and continues (non-blocking).
func (r *Runner) checkScope(ctx context.Context, b *bead.Bead) *prompt.ScopeEstimate {
	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.router == nil {
		return nil
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead for scope check: %v", err)
	}

	// Build scope context
	scopeCtx := &prompt.ScopeContext{
		Bead:       b,
		ParentBead: parent,
	}

	// Render scope prompt
	scopePrompt, err := r.renderer.RenderScope(scopeCtx)
	if err != nil {
		r.log("Warning: failed to render scope prompt: %v", err)
		return nil
	}

	// Select provider via router (phase="scope_check", tier="low")
	p, _ := r.router.Select("scope_check", provider.TierLow)
	if p == nil {
		r.log("Warning: no provider available for scope check")
		return nil
	}

	// Invoke provider with low tier
	result, err := p.Run(ctx, scopePrompt, provider.TierLow)
	if err != nil {
		r.log("Warning: scope check invocation failed: %v", err)
		return nil
	}
	if result == nil {
		r.log("Warning: scope check returned nil result")
		return nil
	}

	if !result.Success {
		r.log("Warning: scope check failed with exit code %d", result.ExitCode)
		return nil
	}

	// Parse the scope estimate
	estimate, err := prompt.ParseScopeEstimate(result.Output)
	if err != nil {
		r.log("Warning: failed to parse scope estimate: %v", err)
		return nil
	}

	return estimate
}
