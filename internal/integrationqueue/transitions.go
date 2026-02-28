package integrationqueue

// CanTransition checks if a transition from one state to another is allowed.
func CanTransition(from, to string) bool {
	if from == "draft" && to == "ready" {
		return true
	}
	return false
}
