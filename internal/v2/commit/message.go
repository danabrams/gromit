package commit

import (
	"fmt"
	"regexp"
	"strconv"
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
	return fmt.Sprintf("[bead:%s/%s/iter:%d] %s", beadID, stageName, iteration, decision)
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

	return Message{
		BeadID:    matches[1],
		StageName: matches[2],
		Iteration: iteration,
		Decision:  matches[4],
	}, nil
}
