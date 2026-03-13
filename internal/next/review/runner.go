package review

import "context"

// ReviewAgent is the interface for invoking LLM review on a facet.
type ReviewAgent interface {
	ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error)
}

// RunnerConfig configures the review runner.
type RunnerConfig struct {
	Facets       []string
	Threshold    Severity
	FacetTiers   map[string]string
	FacetRetries int
}

// RunInput provides data for a single review run.
type RunInput struct {
	DiffSummary   string
	SpecContent   string
	Cycle         int
	PriorFindings []Finding
}

// RunResult holds the outcome of a review run.
type RunResult struct {
	AllFindings         []Finding            `json:"all_findings"`
	BlockingFindings    []Finding            `json:"blocking_findings"`
	HasBlockingFindings bool                 `json:"has_blocking_findings"`
	FindingsByFacet     map[string][]Finding `json:"findings_by_facet"`
	ErroredFacets       map[string]string    `json:"errored_facets"`
}

// NormalizeNilFields maps nil slices/maps to empty values.
func (r *RunResult) NormalizeNilFields() {
	if r.AllFindings == nil {
		r.AllFindings = []Finding{}
	}
	if r.BlockingFindings == nil {
		r.BlockingFindings = []Finding{}
	}
	if r.FindingsByFacet == nil {
		r.FindingsByFacet = map[string][]Finding{}
	}
	if r.ErroredFacets == nil {
		r.ErroredFacets = map[string]string{}
	}
}

// Runner orchestrates per-facet review.
type Runner struct {
	agent  ReviewAgent
	config RunnerConfig
	reg    *Registry
}

// NewRunner creates a review runner.
func NewRunner(agent ReviewAgent, config RunnerConfig) *Runner {
	return &Runner{
		agent:  agent,
		config: config,
		reg:    NewRegistry(),
	}
}

// Run executes all configured facets and assembles findings.
func (r *Runner) Run(ctx context.Context, input RunInput) (*RunResult, error) {
	result := &RunResult{
		FindingsByFacet: make(map[string][]Finding),
		ErroredFacets:   make(map[string]string),
	}

	for _, facetName := range r.config.Facets {
		facetDef, ok := r.reg.Get(facetName)
		if !ok {
			result.ErroredFacets[facetName] = "unknown facet"
			continue
		}

		prompt, err := RenderReviewPrompt(ReviewPromptInput{
			FacetDef:      facetDef,
			DiffSummary:   input.DiffSummary,
			SpecContent:   input.SpecContent,
			PriorFindings: input.PriorFindings,
		})
		if err != nil {
			result.ErroredFacets[facetName] = err.Error()
			continue
		}

		findings, err := r.invokeFacet(ctx, facetName, prompt)
		if err != nil {
			result.ErroredFacets[facetName] = err.Error()
			continue
		}

		// Set cycle and facet on all findings
		for i := range findings {
			findings[i].Cycle = input.Cycle
			findings[i].Facet = facetName
		}

		// Label dispositions on fix cycles
		if input.Cycle > 1 {
			findings = LabelDispositions(findings, input.PriorFindings)
		} else {
			for i := range findings {
				findings[i].Disposition = DispositionNew
			}
		}

		result.FindingsByFacet[facetName] = findings
		result.AllFindings = append(result.AllFindings, findings...)
	}

	// Filter blocking findings: only new findings above threshold block
	for _, f := range result.AllFindings {
		if f.Disposition == DispositionPreExisting {
			continue
		}
		if IsBlocking(r.config.Threshold, f.Severity) {
			result.BlockingFindings = append(result.BlockingFindings, f)
		}
	}
	result.HasBlockingFindings = len(result.BlockingFindings) > 0
	result.NormalizeNilFields()

	return result, nil
}

// invokeFacet calls the agent with retry logic for ParseErrors.
func (r *Runner) invokeFacet(ctx context.Context, facetName, prompt string) ([]Finding, error) {
	maxAttempts := r.config.FacetRetries
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		findings, err := r.agent.ReviewFacet(ctx, facetName, prompt)
		if err == nil {
			return findings, nil
		}
		if !isParseError(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// isParseError checks if an error is a ParseError (retryable).
func isParseError(err error) bool {
	_, ok := err.(*ParseError)
	return ok
}
