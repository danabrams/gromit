package worktree

import "strings"

type sessionCreateFailureClass int

const (
	sessionCreateFailureTerminal sessionCreateFailureClass = iota
	sessionCreateFailureRetryable
	sessionCreateFailureAmbiguousProbe
)

func classifySessionCreateFailure(output string, err error) sessionCreateFailureClass {
	if err == nil {
		return sessionCreateFailureTerminal
	}

	msg := sessionCreateErrorMessage(output, err)
	if strings.Contains(msg, "a branch named") && strings.Contains(msg, "already exists") {
		return sessionCreateFailureRetryable
	}
	if strings.Contains(msg, "already checked out") {
		return sessionCreateFailureRetryable
	}
	if strings.Contains(msg, "already used by worktree") {
		return sessionCreateFailureRetryable
	}
	if strings.Contains(msg, "already registered as a worktree") {
		return sessionCreateFailureRetryable
	}
	if strings.Contains(msg, "cannot lock ref") && strings.Contains(msg, "refs/heads/") {
		return sessionCreateFailureAmbiguousProbe
	}

	return sessionCreateFailureTerminal
}

func sessionCreateErrorMessage(output string, err error) string {
	msg := strings.ToLower(strings.TrimSpace(output))
	if msg == "" {
		msg = strings.ToLower(err.Error())
	}
	return msg
}
