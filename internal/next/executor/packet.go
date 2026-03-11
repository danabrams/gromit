package executor

import (
	"fmt"
	"strings"
)

// TaskPacketInput holds all the data needed to compile a task prompt packet.
type TaskPacketInput struct {
	SpecPacket   string
	TaskID       string
	Objective    string
	ProofChecks  []string
	ExpectedArea []string
	PriorContext string
}

// CompileTaskPacket renders a task prompt packet from the given input.
func CompileTaskPacket(input TaskPacketInput) (string, error) {
	var b strings.Builder

	b.WriteString("## Spec Context\n\n")
	b.WriteString(input.SpecPacket)
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("## Task %s\n\n", input.TaskID))

	b.WriteString("### Objective\n\n")
	b.WriteString(input.Objective)
	b.WriteString("\n\n")

	if len(input.ProofChecks) > 0 {
		b.WriteString("### Proof Checks\n\n")
		for _, check := range input.ProofChecks {
			b.WriteString(fmt.Sprintf("- %s\n", check))
		}
		b.WriteString("\n")
	}

	if len(input.ExpectedArea) > 0 {
		b.WriteString("### Expected Area\n\n")
		for _, area := range input.ExpectedArea {
			b.WriteString(fmt.Sprintf("- %s\n", area))
		}
		b.WriteString("\n")
	}

	if input.PriorContext != "" {
		b.WriteString("### Prior Context\n\n")
		b.WriteString(input.PriorContext)
		b.WriteString("\n")
	}

	return b.String(), nil
}
