package andon

import (
	"fmt"
	"strings"
)

const requiredEscalationOptions = 3

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
	return firstValidationError(
		requiredString("failed_command", packet.FailedCommand),
		requiredString("error_excerpt", packet.ErrorExcerpt),
		requiredAttempts("l1_attempts", packet.L1Attempts),
		requiredAttempts("l2_attempts", packet.L2Attempts),
		requiredString("state_snapshot", packet.StateSnapshot),
		requiredString("risk_level", string(packet.RiskLevel)),
		requiredString("recommendation", packet.Recommendation),
	)
}

// FormatEscalationPacket renders a packet for CLI/UI consumption.
func FormatEscalationPacket(packet EscalationPacket) (string, error) {
	if err := ValidateEscalationPacket(packet); err != nil {
		return "", err
	}

	if len(packet.Options) != requiredEscalationOptions {
		return "", fmt.Errorf("exactly three options are required")
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

func requiredString(fieldName, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

func requiredAttempts(fieldName string, attempts []EscalationAttempt) error {
	if len(attempts) == 0 {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

func firstValidationError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
