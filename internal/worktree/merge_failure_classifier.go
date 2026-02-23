package worktree

import "strings"

type mergeFailureClass int

const (
	mergeFailureNonConflict mergeFailureClass = iota
	mergeFailureConflict
)

type mergeFailureInput struct {
	Output          string
	Err             error
	MergeInProgress bool
}

type mergeFailureDecision struct {
	Class         mergeFailureClass
	ExitCode      int
	ExitCodeKnown bool
}

type exitCoder interface {
	ExitCode() int
}

func classifyMergeFailure(input mergeFailureInput) mergeFailureDecision {
	decision := mergeFailureDecision{}
	if input.Err == nil {
		return decision
	}

	if coded, ok := input.Err.(exitCoder); ok {
		decision.ExitCodeKnown = true
		decision.ExitCode = coded.ExitCode()
	}

	msg := strings.TrimSpace(input.Output)
	errMsg := strings.TrimSpace(input.Err.Error())
	if errMsg != "" {
		if msg != "" {
			msg += "\n"
		}
		msg += errMsg
	}

	for _, line := range strings.Split(msg, "\n") {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(normalized, "conflict ") ||
			strings.HasPrefix(normalized, "conflict(") ||
			strings.HasPrefix(normalized, "conflict:") {
			decision.Class = mergeFailureConflict
			return decision
		}
		if strings.Contains(normalized, "automatic merge failed") {
			decision.Class = mergeFailureConflict
			return decision
		}
	}

	if input.MergeInProgress {
		decision.Class = mergeFailureConflict
	}

	return decision
}
