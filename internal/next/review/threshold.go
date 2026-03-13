package review

// IsBlocking returns true if a finding at the given severity should block
// at the given threshold. Info findings never block regardless of threshold.
func IsBlocking(threshold, findingSeverity Severity) bool {
	if findingSeverity == SeverityInfo {
		return false
	}
	return findingSeverity.Rank() >= threshold.Rank()
}

// FilterBlockingFindings returns the subset of findings that are blocking
// at the given threshold.
func FilterBlockingFindings(findings []Finding, threshold Severity) []Finding {
	var result []Finding
	for _, f := range findings {
		if IsBlocking(threshold, f.Severity) {
			result = append(result, f)
		}
	}
	return result
}
