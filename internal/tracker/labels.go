package tracker

const specLabelPrefix = "spec:"

// SpecLabelPrefix identifies labels that map beads back to specs (e.g., "spec:auth").
// It reuses specLabelPrefix so internal callers share the same literal.
const SpecLabelPrefix = specLabelPrefix

// SpecLabelFor formats a spec label using the shared prefix.
func SpecLabelFor(specName string) string {
	return specLabelPrefix + specName
}
