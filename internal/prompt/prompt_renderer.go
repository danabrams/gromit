package prompt

// PromptRenderer defines the interface for rendering prompts with support for coverage validation.
type PromptRenderer interface {
	// RenderCoverageValidation renders the coverage validation prompt.
	RenderCoverageValidation(ctx *CoverageValidationContext) (string, error)
}
