package bead

// IsAtomic classifies a bead as atomic based on depth limit, atomic:true label,
// and single-target heuristics.
//
// A bead is atomic if any of these conditions are true:
// 1. The bead has the atomic:true label
// 2. depth >= maxDepth (at or beyond max decomposition depth)
// 3. The bead targets a single file/function (single-target heuristic)
func IsAtomic(bead *Bead, depth, maxDepth int) bool {
	if bead == nil {
		return false
	}

	// Check for atomic:true label
	if HasLabel(bead.Labels, "atomic:true") {
		return true
	}

	return false
}
