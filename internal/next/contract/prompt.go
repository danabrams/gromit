package contract

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed prompt.txt
var contractPromptText string

// ContractPromptInput holds the context for rendering a contract-writing prompt.
type ContractPromptInput struct {
	SpecPacket string
	Scenarios  []SpecScenario
}

var contractPromptTmpl = template.Must(template.New("contract").Parse(contractPromptText))

// RenderContractPrompt renders a prompt instructing the LLM to translate
// Given/When/Then scenarios into YAML contract assertions.
func RenderContractPrompt(input ContractPromptInput) (string, error) {
	var buf bytes.Buffer
	if err := contractPromptTmpl.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}
