package integrationqueue

// CanTransition checks if a transition from one state to another is allowed.
func CanTransition(from, to string) bool {
	if from == "draft" && to == "ready" {
		return true
	}
	if from == "ready" && to == "integrating" {
		return true
	}
	if from == "integrating" && to == "merged" {
		return true
	}
	if from == "integrating" && to == "conflict" {
		return true
	}
	return false
}
