package prompt

const (
	SectionRules              = "rules"
	SectionClaudeMD           = "claude_md"
	SectionSpec               = "spec"
	SectionConfirmedLearnings = "confirmed_learnings"
	SectionRecentLearnings    = "recent_learnings"
	SectionTaskIdentity       = "task_identity"
	SectionDiff               = "diff"
	SectionFailureContext     = "failure_context"
	SectionTemplateStatic     = "template_static"
	SectionSkillInstructions  = "skill_instructions"
	SectionPlanBody           = "plan_body"
	SectionRunStats           = "run_stats"
	SectionBeadStats          = "bead_stats"

	SourceBucketTemplate       = "template"
	SourceBucketRules          = "rules"
	SourceBucketLearnings      = "learnings"
	SourceBucketToolOutput     = "tool_output"
	SourceBucketContextPayload = "context_payload"
)

// PromptDiagnostics captures estimated token attribution and reconciliation.
type PromptDiagnostics struct {
	PromptType         string         `json:"prompt_type"`
	EstimatedTokens    int            `json:"estimated_tokens"`
	SectionTokens      map[string]int `json:"section_tokens"`
	SourceBucketTokens map[string]int `json:"source_bucket_tokens,omitempty"`
	BudgetMaxChars     int            `json:"budget_max_chars,omitempty"`
	ShapeActions       []string       `json:"shape_actions,omitempty"`
	PreShapeTokens     int            `json:"pre_shape_tokens,omitempty"`
	PostShapeTokens    int            `json:"post_shape_tokens,omitempty"`
	ReportedTokens     int            `json:"reported_tokens,omitempty"`
	TokenDelta         int            `json:"token_delta,omitempty"`
	TokenDeltaPct      float64        `json:"token_delta_pct,omitempty"`
}

// NewDiagnostics builds diagnostics from section-level token estimates.
func NewDiagnostics(promptType string, sectionTokens map[string]int) *PromptDiagnostics {
	tokens := make(map[string]int, len(sectionTokens))
	estimated := 0
	for section, count := range sectionTokens {
		tokens[section] = count
		estimated += count
	}

	return &PromptDiagnostics{
		PromptType:         promptType,
		EstimatedTokens:    estimated,
		SectionTokens:      tokens,
		SourceBucketTokens: estimateSourceBucketTokens(tokens),
	}
}

func estimateSourceBucketTokens(sectionTokens map[string]int) map[string]int {
	buckets := make(map[string]int)
	for section, tokens := range sectionTokens {
		bucket := SourceBucketContextPayload
		switch section {
		case SectionTemplateStatic:
			bucket = SourceBucketTemplate
		case SectionRules, SectionClaudeMD, SectionSkillInstructions:
			bucket = SourceBucketRules
		case SectionConfirmedLearnings, SectionRecentLearnings:
			bucket = SourceBucketLearnings
		case SectionDiff, SectionFailureContext, SectionRunStats, SectionBeadStats:
			bucket = SourceBucketToolOutput
		}

		buckets[bucket] += tokens
	}
	return buckets
}

// Reconcile updates reconciliation fields from provider-reported input tokens.
func (d *PromptDiagnostics) Reconcile(reportedTokens int) {
	if d == nil {
		return
	}

	d.ReportedTokens = reportedTokens
	d.TokenDelta = d.EstimatedTokens - reportedTokens
	if reportedTokens <= 0 {
		d.TokenDeltaPct = 0
		return
	}

	d.TokenDeltaPct = (float64(d.TokenDelta) / float64(reportedTokens)) * 100
}
