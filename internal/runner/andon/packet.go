package andon

import (
	"fmt"
	"strings"
)

// RiskLevel identifies escalation impact severity.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// EscalationAttempt describes one attempt and result at a recovery level.
type EscalationAttempt struct {
	Summary string
	Outcome string
}

// EscalationOption describes one actionable option with explicit tradeoff.
type EscalationOption struct {
	Title    string
	Tradeoff string
}

// EscalationPacket is the data model for Andon escalation handoff.
type EscalationPacket struct {
	FailedCommand  string
	ErrorExcerpt   string
	L1Attempts     []EscalationAttempt
	L2Attempts     []EscalationAttempt
	StateSnapshot  string
	RiskLevel      RiskLevel
	Options        []EscalationOption
	Recommendation string
}

// ValidateEscalationPacket validates required escalation packet fields.
func ValidateEscalationPacket(packet EscalationPacket) error {
	if strings.TrimSpace(packet.FailedCommand) == "" {
		return fmt.Errorf("failed_command is required")
	}
	if strings.TrimSpace(packet.ErrorExcerpt) == "" {
		return fmt.Errorf("error_excerpt is required")
	}
	if len(packet.L1Attempts) == 0 {
		return fmt.Errorf("l1_attempts is required")
	}
	if len(packet.L2Attempts) == 0 {
		return fmt.Errorf("l2_attempts is required")
	}
	if strings.TrimSpace(packet.StateSnapshot) == "" {
		return fmt.Errorf("state_snapshot is required")
	}
	if strings.TrimSpace(string(packet.RiskLevel)) == "" {
		return fmt.Errorf("risk_level is required")
	}
	if strings.TrimSpace(packet.Recommendation) == "" {
		return fmt.Errorf("recommendation is required")
	}

	return nil
}

// FormatEscalationPacket renders a packet for CLI/UI consumption.
func FormatEscalationPacket(packet EscalationPacket) (string, error) {
	if err := ValidateEscalationPacket(packet); err != nil {
		return "", err
	}

	if len(packet.Options) != 3 {
		return "", fmt.Errorf("exactly three options are required")
	}

	if strings.TrimSpace(packet.Recommendation) == "" {
		return "", fmt.Errorf("recommendation is required")
	}

	var output strings.Builder
	fmt.Fprintf(&output, "Failed Command: %s\n", packet.FailedCommand)
	fmt.Fprintf(&output, "Error Excerpt: %s\n", packet.ErrorExcerpt)
	fmt.Fprintf(&output, "State Snapshot: %s\n", packet.StateSnapshot)
	fmt.Fprintf(&output, "Risk Level: %s\n", packet.RiskLevel)
	output.WriteString("L1 Attempts:\n")
	for _, attempt := range packet.L1Attempts {
		fmt.Fprintf(&output, "- %s | Outcome: %s\n", attempt.Summary, attempt.Outcome)
	}
	output.WriteString("L2 Attempts:\n")
	for _, attempt := range packet.L2Attempts {
		fmt.Fprintf(&output, "- %s | Outcome: %s\n", attempt.Summary, attempt.Outcome)
	}
	output.WriteString("Options:\n")
	for _, option := range packet.Options {
		fmt.Fprintf(&output, "- %s\n  Tradeoff: %s\n", option.Title, option.Tradeoff)
	}
	fmt.Fprintf(&output, "Recommendation: %s\n", packet.Recommendation)

	return output.String(), nil
}
