package pipeline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CommitInfo holds the parsed fields from a structured commit message.
type CommitInfo struct {
	BeadID    string // empty string for spec-level stages
	StageName string
	Iteration int
	Decision  string
}

// FormatCommitMessage produces a structured commit message.
// If beadID is empty, the message uses "spec" as the scope.
func FormatCommitMessage(beadID, stageName string, iteration int, decision string) string {
	scope := "spec"
	if beadID != "" {
		scope = "bead:" + beadID
	}
	return fmt.Sprintf("[%s/%s/iter:%d] %s", scope, stageName, iteration, decision)
}

var commitMessageRe = regexp.MustCompile(`^\[(bead:([^/]+)|spec)/([^/]+)/iter:(\d+)\]\s+(.+)$`)

// ParseCommitMessage extracts structured fields from a commit message.
// Returns false if the message does not match the expected format.
func ParseCommitMessage(msg string) (CommitInfo, bool) {
	m := commitMessageRe.FindStringSubmatch(strings.TrimSpace(msg))
	if m == nil {
		return CommitInfo{}, false
	}
	iter, _ := strconv.Atoi(m[4])
	return CommitInfo{
		BeadID:    m[2], // empty when scope is "spec"
		StageName: m[3],
		Iteration: iter,
		Decision:  m[5],
	}, true
}
