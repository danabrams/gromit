package tracker

// SpecLabelPrefix identifies labels that map beads back to specs (e.g., "spec:auth").
const SpecLabelPrefix = "spec:"

// SpecLabelFor formats a spec label using the shared prefix.
func SpecLabelFor(specName string) string {
	return SpecLabelPrefix + specName
}
