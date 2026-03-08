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

// BuildCommitMessage renders the structured stage commit message string.
func BuildCommitMessage(m CommitMessage) string {
	beadID := m.BeadID
	if beadID == "" {
		beadID = "spec"
	}
	return fmt.Sprintf("[bead:%s/%s/iter:%d] %s", beadID, m.StageName, m.Iteration, m.Decision)
}

// Build renders the structured stage commit message string.
func (m CommitMessage) Build() string {
	return BuildCommitMessage(m)
}

var commitMessageRe = regexp.MustCompile(`^\[bead:([^/]+)/([^/]+)/iter:(\d+)\]\s+(.+)$`)
var commitMessageBeadIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var commitMessageStageNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// ParseCommitMessage parses a structured stage commit message into machine fields.
func ParseCommitMessage(msg string) (CommitMessage, bool) {
	trimmed := strings.TrimSpace(msg)
	if lineBreak := strings.IndexByte(trimmed, '\n'); lineBreak >= 0 {
		trimmed = strings.TrimSpace(trimmed[:lineBreak])
	}

	matches := commitMessageRe.FindStringSubmatch(trimmed)
	if matches == nil {
		return CommitMessage{}, false
	}
	beadID := matches[1]
	if !commitMessageBeadIDRe.MatchString(beadID) {
		return CommitMessage{}, false
	}
	stageName := matches[2]
	if !commitMessageStageNameRe.MatchString(stageName) {
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
		BeadID:    beadID,
		StageName: stageName,
		Iteration: iteration,
		Decision:  matches[4],
	}, true
}
