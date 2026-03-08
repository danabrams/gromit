package loop

import "fmt"

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
