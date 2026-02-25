package escalation

const sameScopeRetryBlockedMessage = "Same-scope retry blocked: timeout requires decomposition or escalation decision"
const partialDecompositionStateMessage = "Partial/unsafe decomposition state: retry or escalate before continuing"

type sameScopeRetryBlockedError struct{}

func (sameScopeRetryBlockedError) Error() string {
	return sameScopeRetryBlockedMessage
}

type partialDecompositionStateError struct{}

func (partialDecompositionStateError) Error() string {
	return partialDecompositionStateMessage
}

var ErrSameScopeRetryBlocked error = sameScopeRetryBlockedError{}
var ErrPartialDecompositionState error = partialDecompositionStateError{}
