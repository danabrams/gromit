package prompt

import (
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

// ShapeRedPhaseContext trims low-signal context while preserving acceptance and guardrails.
func (r *Renderer) ShapeRedPhaseContext(ctx *Context) *Context {
	shaped := cloneMethodologyContext(ctx)
	if shaped == nil {
		return nil
	}

	shaped.ClaudeMD = ""
	shaped.ConfirmedLearnings = []learnings.Learning{}
	shaped.RecentLearnings = []learnings.Learning{}
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

	shaped.ClaudeMD = ""
	shaped.ConfirmedLearnings = []learnings.Learning{}
	shaped.RecentLearnings = []learnings.Learning{}
	shaped.normalizeNilFields()

	return shaped
}

// ShapeRefactorPhaseContext keeps constraints and guardrails while dropping build payload.
func (r *Renderer) ShapeRefactorPhaseContext(ctx *Context) *Context {
	shaped := cloneMethodologyContext(ctx)
	if shaped == nil {
		return nil
	}

	shaped.ClaudeMD = ""
	shaped.ConfirmedLearnings = []learnings.Learning{}
	shaped.RecentLearnings = []learnings.Learning{}
	shaped.RecentValidationFailures = []string{}
	if shaped.Bead != nil {
		shaped.Bead.ExpectedOutputs = []string{}
	}
	ensureBehaviorGuardrails(shaped)
	shaped.normalizeNilFields()

	return shaped
}

func ensureBehaviorGuardrails(ctx *Context) {
	const guardrail = "Do not change behavior."
	if !strings.Contains(strings.ToLower(ctx.Rules), "do not change behavior") {
		if strings.TrimSpace(ctx.Rules) == "" {
			ctx.Rules = guardrail
		} else {
			ctx.Rules = strings.TrimSpace(ctx.Rules) + "\n- " + guardrail
		}
	}

	lower := strings.ToLower(ctx.FailureContext + "\n" + ctx.PrevFailure)
	if strings.Contains(lower, "do not change behavior") {
		return
	}
	if strings.TrimSpace(ctx.FailureContext) == "" {
		ctx.FailureContext = guardrail
		return
	}
	ctx.FailureContext = strings.TrimSpace(ctx.FailureContext) + "\n" + guardrail
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
	const guardrail = "TEST-ONLY guardrails: change tests only and do not modify implementation code in this phase."
	lower := strings.ToLower(ctx.FailureContext + "\n" + ctx.PrevFailure)
	if strings.Contains(lower, "test-only") && strings.Contains(lower, "implementation") {
		return
	}
	if strings.TrimSpace(ctx.FailureContext) == "" {
		ctx.FailureContext = guardrail
		return
	}
	ctx.FailureContext = strings.TrimSpace(ctx.FailureContext) + "\n" + guardrail
}
