package acceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

const (
	proseFallbackCriterionPlaceholder = "(parsed from prose)"
	proseRationaleMaxLen              = 500
	parseErrorRawOutputLimit          = 500
)

// validStatuses is the set of allowed CriterionResult.Status values.
var validStatuses = map[string]bool{
	StatusPass:    true,
	StatusFail:    true,
	StatusUnclear: true,
}

type proseSignal struct {
	status  string
	pattern *regexp.Regexp
}

type proseSignalMatch struct {
	status string
	start  int
	end    int
}

var (
	proseSignalPatterns []proseSignal
)

var proseStatusSignalsOrdered = []struct {
	status  string
	signals []string
}{
	{StatusPass, []string{
		"pass",
		"PASS",
		"passed",
		"criterion is met",
		"assessment: pass",
	}},
	{StatusFail, []string{
		"fail",
		"failed",
		"criterion is not met",
		"not satisfied",
		"assessment: fail",
	}},
	{StatusUnclear, []string{
		"unclear",
		"insufficient evidence",
		"cannot determine",
		"assessment: unclear",
	}},
}

func init() {
	for _, entry := range proseStatusSignalsOrdered {
		for _, signal := range entry.signals {
			pattern := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(signal)))
			proseSignalPatterns = append(proseSignalPatterns, proseSignal{status: entry.status, pattern: pattern})
		}
	}
}

// Compile-time interface check.
var _ AcceptAgent = (*ProviderAcceptAgent)(nil)

// ProviderAcceptAgent satisfies AcceptAgent by delegating to an llmadapter.Invoker.
type ProviderAcceptAgent struct {
	invoker llmadapter.Invoker
}

// NewProviderAcceptAgent creates a ProviderAcceptAgent backed by the given invoker.
func NewProviderAcceptAgent(invoker llmadapter.Invoker) *ProviderAcceptAgent {
	return &ProviderAcceptAgent{invoker: invoker}
}

// EvaluateCriterion sends the prompt to the LLM and parses a CriterionResult from the response.
func (a *ProviderAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	result, err := a.invoker.Invoke(ctx, prompt)
	if err != nil {
		return CriterionResult{}, err
	}
	if result == nil {
		return CriterionResult{}, fmt.Errorf("acceptor: provider returned nil result")
	}
	return ParseCriterionResult(result.Output)
}

// ParseCriterionResult extracts JSON from raw LLM output, unmarshals it into
// a CriterionResult, and normalizes nil fields. It is exported so callers can
// reuse the parsing logic independently.
func ParseCriterionResult(output string) (CriterionResult, error) {
	extracted := llmadapter.ExtractJSON(output)
	var cr CriterionResult
	if err := json.Unmarshal([]byte(extracted), &cr); err != nil {
		rawSnippet := truncateString(output, parseErrorRawOutputLimit)
		if strings.ContainsAny(output, "{[") {
			// Output contains JSON markers but failed to parse — return error directly.
			return CriterionResult{}, &ParseError{
				Msg: fmt.Sprintf("parsing criterion result: %s; raw output: %s", err.Error(), rawSnippet),
			}
		}
		// Pure prose — attempt prose fallback.
		fallback, perr := parseCriterionFromProse(output)
		if perr == nil {
			fallback.NormalizeNilFields()
			return fallback, nil
		}
		return CriterionResult{}, &ParseError{
			Msg: fmt.Sprintf("parsing criterion result: %s; raw output: %s", err.Error(), rawSnippet),
		}
	}
	if cr.Criterion == "" {
		return CriterionResult{}, &ParseError{Msg: "missing required field: criterion"}
	}
	if cr.Status == "" {
		return CriterionResult{}, &ParseError{Msg: "missing required field: status"}
	}
	if !validStatuses[cr.Status] {
		return CriterionResult{}, &ParseError{Msg: fmt.Sprintf("invalid status %q: must be pass, fail, or unclear", cr.Status)}
	}
	if cr.Rationale == "" {
		return CriterionResult{}, &ParseError{Msg: "missing required field: rationale"}
	}
	cr.NormalizeNilFields()
	return cr, nil
}

func parseCriterionFromProse(output string) (CriterionResult, error) {
	matches := collectProseSignalMatches(output)
	if len(matches) == 0 {
		return CriterionResult{}, fmt.Errorf("prose parsing: no status signal")
	}

	statusSet := make(map[string]struct{})
	for _, match := range matches {
		statusSet[match.status] = struct{}{}
	}

	if len(statusSet) == 1 {
		var status string
		for s := range statusSet {
			status = s
		}
		cr := CriterionResult{
			Criterion:    proseFallbackCriterionPlaceholder,
			Status:       status,
			Rationale:    truncateString(output, proseRationaleMaxLen),
			EvidenceRefs: []string{},
		}
		cr.NormalizeNilFields()
		return cr, nil
	}
	statuses := make([]string, 0, len(statusSet))
	for status := range statusSet {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	return CriterionResult{}, fmt.Errorf("prose parsing: conflicting status signals %v", statuses)
}

func collectProseSignalMatches(output string) []proseSignalMatch {
	var matches []proseSignalMatch
	for _, signal := range proseSignalPatterns {
		locs := signal.pattern.FindAllStringIndex(output, -1)
		for _, loc := range locs {
			matches = append(matches, proseSignalMatch{
				status: signal.status,
				start:  loc[0],
				end:    loc[1],
			})
		}
	}
	return matches
}

func truncateString(s string, limit int) string {
	if limit <= 0 || s == "" {
		return ""
	}
	count := 0
	for i := range s {
		if count == limit {
			return s[:i]
		}
		count++
	}
	return s
}
