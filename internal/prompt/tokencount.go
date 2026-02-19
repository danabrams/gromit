package prompt

const charsPerToken = 4

// EstimateTokens returns a heuristic token estimate based on character count.
// It uses a chars/4 approximation and rounds up so non-empty input is always
// estimated as at least one token.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	return (len(text) + charsPerToken - 1) / charsPerToken
}

// EstimateSectionTokens returns per-section token estimates using EstimateTokens.
func EstimateSectionTokens(sections map[string]string) map[string]int {
	estimates := make(map[string]int, len(sections))
	for name, text := range sections {
		estimates[name] = EstimateTokens(text)
	}

	return estimates
}
