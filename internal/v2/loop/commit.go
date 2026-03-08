package loop

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CommitMessage describes the structured stage commit message fields.
type CommitMessage struct {
	BeadID    string
	StageName string
	Iteration int
	Decision  string
}

// Build renders the structured stage commit message string.
func (m CommitMessage) Build() string {
	beadID := m.BeadID
	if beadID == "" {
		beadID = "spec"
	}
	return fmt.Sprintf("[bead:%s/%s/iter:%d] %s", beadID, m.StageName, m.Iteration, m.Decision)
}

var commitMessageRe = regexp.MustCompile(`^\[bead:([^/]+)/([^/]+)/iter:(\d+)\]\s+(.+)$`)

// ParseCommitMessage parses a structured stage commit message into machine fields.
func ParseCommitMessage(msg string) (CommitMessage, bool) {
	matches := commitMessageRe.FindStringSubmatch(strings.TrimSpace(msg))
	if matches == nil {
		return CommitMessage{}, false
	}

	iteration, err := strconv.Atoi(matches[3])
	if err != nil {
		return CommitMessage{}, false
	}
	if iteration <= 0 {
		return CommitMessage{}, false
	}

	return CommitMessage{
		BeadID:    matches[1],
		StageName: matches[2],
		Iteration: iteration,
		Decision:  matches[4],
	}, true
}
