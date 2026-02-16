package prompt

import (
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

const (
	behaviorGuardrailPhrase = "Do not change behavior."
	behaviorGuardrailNeedle = "do not change behavior"
	testOnlyGuardrailPhrase = "TEST-ONLY guardrails: change tests only and do not modify implementation code in this phase."
)

// ShapeRedPhaseContext trims low-signal context while preserving acceptance and guardrails.
func (r *Renderer) ShapeRedPhaseContext(ctx *Context) *Context {
	shaped := cloneMethodologyContext(ctx)
	if shaped == nil {
		return nil
	}

	trimSharedLowSignalContext(shaped)
	ensureTestOnlyGuardrails(shaped)
	shaped.normalizeNilFields()

	return shaped
}

// ShapeGreenPhaseContext preserves required behavior and failure signal while trimming noise.
func (r *Renderer) ShapeGreenPhaseContext(ctx *Context) *Context {
	shaped := cloneMethodologyContext(ctx)
	if shaped == nil {
		return nil
	}

	trimSharedLowSignalContext(shaped)
	shaped.normalizeNilFields()

	return shaped
}

// ShapeRefactorPhaseContext keeps constraints and guardrails while dropping build payload.
func (r *Renderer) ShapeRefactorPhaseContext(ctx *Context) *Context {
	shaped := cloneMethodologyContext(ctx)
	if shaped == nil {
		return nil
	}

	trimSharedLowSignalContext(shaped)
	shaped.RecentValidationFailures = []string{}
	if shaped.Bead != nil {
		shaped.Bead.ExpectedOutputs = []string{}
	}
	ensureBehaviorGuardrails(shaped)
	shaped.normalizeNilFields()

	return shaped
}

func trimSharedLowSignalContext(ctx *Context) {
	ctx.ClaudeMD = ""
	ctx.ConfirmedLearnings = []learnings.Learning{}
	ctx.RecentLearnings = []learnings.Learning{}
}

func ensureBehaviorGuardrails(ctx *Context) {
	if !strings.Contains(strings.ToLower(ctx.Rules), behaviorGuardrailNeedle) {
		if strings.TrimSpace(ctx.Rules) == "" {
			ctx.Rules = behaviorGuardrailPhrase
		} else {
			ctx.Rules = strings.TrimSpace(ctx.Rules) + "\n- " + behaviorGuardrailPhrase
		}
	}

	lower := strings.ToLower(ctx.FailureContext + "\n" + ctx.PrevFailure)
	if strings.Contains(lower, behaviorGuardrailNeedle) {
		return
	}
	if strings.TrimSpace(ctx.FailureContext) == "" {
		ctx.FailureContext = behaviorGuardrailPhrase
		return
	}
	ctx.FailureContext = strings.TrimSpace(ctx.FailureContext) + "\n" + behaviorGuardrailPhrase
}

func cloneMethodologyContext(ctx *Context) *Context {
	if ctx == nil {
		return nil
	}

	cloned := *ctx
	cloned.Bead = cloneBead(ctx.Bead)
	cloned.ParentBead = cloneBead(ctx.ParentBead)
	cloned.ConfirmedLearnings = append([]learnings.Learning{}, ctx.ConfirmedLearnings...)
	cloned.RecentLearnings = append([]learnings.Learning{}, ctx.RecentLearnings...)
	cloned.RecentValidationFailures = append([]string{}, ctx.RecentValidationFailures...)
	cloned.normalizeNilFields()

	return &cloned
}

func cloneBead(b *bead.Bead) *bead.Bead {
	if b == nil {
		return nil
	}

	cloned := *b
	cloned.Labels = append([]string{}, b.Labels...)
	cloned.ExpectedOutputs = append([]string{}, b.ExpectedOutputs...)
	if cloned.Labels == nil {
		cloned.Labels = []string{}
	}
	if cloned.ExpectedOutputs == nil {
		cloned.ExpectedOutputs = []string{}
	}

	return &cloned
}

func ensureTestOnlyGuardrails(ctx *Context) {
	lower := strings.ToLower(ctx.FailureContext + "\n" + ctx.PrevFailure)
	if strings.Contains(lower, "test-only") && strings.Contains(lower, "implementation") {
		return
	}
	if strings.TrimSpace(ctx.FailureContext) == "" {
		ctx.FailureContext = testOnlyGuardrailPhrase
		return
	}
	ctx.FailureContext = strings.TrimSpace(ctx.FailureContext) + "\n" + testOnlyGuardrailPhrase
}
