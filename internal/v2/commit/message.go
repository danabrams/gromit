package commit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Message holds parsed structured commit message fields.
type Message struct {
	BeadID    string
	StageName string
	Iteration int
	Decision  string
}

// FormatMessage encodes a structured commit message.
func FormatMessage(beadID, stageName string, iteration int, decision string) string {
	normalizedBeadID := strings.TrimSpace(beadID)
	if normalizedBeadID == "" || normalizedBeadID == "spec" {
		return fmt.Sprintf("[spec/%s/iter:%d] %s", stageName, iteration, decision)
	}
	return fmt.Sprintf("[bead:%s/%s/iter:%d] %s", normalizedBeadID, stageName, iteration, decision)
}

var messageRe = regexp.MustCompile(`^\[bead:([^/]+)/([^/]+)/iter:(\d+)\]\s+(.+)$`)

// ParseMessage decodes a structured commit message into its components.
func ParseMessage(msg string) (Message, error) {
	matches := messageRe.FindStringSubmatch(msg)
	if matches == nil {
		return Message{}, fmt.Errorf("invalid commit message format")
	}

	iteration, err := strconv.Atoi(matches[3])
	if err != nil {
		return Message{}, fmt.Errorf("invalid iteration: %w", err)
	}

	parsed := Message{
		BeadID:    matches[1],
		StageName: matches[2],
		Iteration: iteration,
		Decision:  matches[4],
	}
	if err := validateRequiredFields(parsed); err != nil {
		return Message{}, err
	}
	return parsed, nil
}

func validateRequiredFields(msg Message) error {
	if strings.TrimSpace(msg.BeadID) == "" {
		return fmt.Errorf("bead ID is required")
	}
	if strings.TrimSpace(msg.StageName) == "" {
		return fmt.Errorf("stage name is required")
	}
	if msg.Iteration <= 0 {
		return fmt.Errorf("iteration must be greater than zero")
	}
	if strings.TrimSpace(msg.Decision) == "" {
		return fmt.Errorf("decision is required")
	}
	return nil
}
