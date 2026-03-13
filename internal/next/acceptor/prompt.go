package acceptor

import (
	"bytes"
	"text/template"
)

// AcceptancePromptInput holds the context for rendering an acceptance evaluation prompt.
type AcceptancePromptInput struct {
	Criterion         string
	DiffSummary       string
	TaskResults       string
	ValidationResults string
	ReviewFindings    string
}

var acceptancePromptTmpl = template.Must(template.New("acceptance").Parse(`You are evaluating whether an acceptance criterion has been met.

## Criterion
{{.Criterion}}

## Evidence

{{- if .DiffSummary}}

### Diff Summary
{{.DiffSummary}}
{{- end}}

{{- if .TaskResults}}

### Task Results
{{.TaskResults}}
{{- end}}

{{- if .ValidationResults}}

### Validation Results
{{.ValidationResults}}
{{- end}}

{{- if .ReviewFindings}}

### Review Findings
{{.ReviewFindings}}
{{- end}}

## Instructions

Evaluate the criterion above against the provided evidence. Respond with one of three statuses:

- **pass**: The criterion is clearly satisfied by the evidence.
- **fail**: The criterion is clearly NOT satisfied — implementation is missing or incorrect.
- **unclear**: There is insufficient evidence to determine whether the criterion is met. This typically means tests or other proof are missing, not that the implementation is wrong.

Respond with a JSON object:
{
  "criterion": "<the criterion text>",
  "status": "pass|fail|unclear",
  "rationale": "<brief explanation>",
  "evidence_refs": ["<relevant file paths or evidence references>"]
}
`))

// RenderAcceptancePrompt renders a prompt for evaluating a single acceptance criterion.
func RenderAcceptancePrompt(input AcceptancePromptInput) (string, error) {
	var buf bytes.Buffer
	if err := acceptancePromptTmpl.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}
