package review

import (
	"bytes"
	"text/template"
)

// ReviewPromptInput provides data for rendering a facet's review prompt.
type ReviewPromptInput struct {
	FacetDef      FacetDef
	DiffSummary   string
	SpecContent   string
	PriorFindings []Finding
}

// RenderReviewPrompt renders the facet's prompt template with the given input.
func RenderReviewPrompt(input ReviewPromptInput) (string, error) {
	tmpl, err := template.New("review").Parse(input.FacetDef.PromptTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}
