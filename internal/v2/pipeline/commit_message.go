package pipeline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CommitInfo holds the parsed fields from a structured commit message.
type CommitInfo struct {
	BeadID    string
	StageName string
	Iteration int
	Decision  string
}

// FormatCommitMessage produces a structured commit message.
// If beadID is empty, the message uses "spec" as the scope.
func FormatCommitMessage(beadID, stageName string, iteration int, decision string) string {
	if beadID == "" {
		beadID = "spec"
	}
	return fmt.Sprintf("[bead:%s/%s/iter:%d] %s", beadID, stageName, iteration, decision)
}

var commitMessageRe = regexp.MustCompile(`^\[bead:([^/]+)/([^/]+)/iter:(\d+)\]\s+(.+)$`)

// ParseCommitMessage extracts structured fields from a commit message.
// Returns false if the message does not match the expected format.
func ParseCommitMessage(msg string) (CommitInfo, bool) {
	m := commitMessageRe.FindStringSubmatch(strings.TrimSpace(msg))
	if m == nil {
		return CommitInfo{}, false
	}
	iter, _ := strconv.Atoi(m[3])
	return CommitInfo{
		BeadID:    m[1],
		StageName: m[2],
		Iteration: iter,
		Decision:  m[4],
	}, true
}
