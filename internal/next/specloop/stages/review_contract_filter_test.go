package stages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/review"
	"gopkg.in/yaml.v3"
)

func TestFilterContractContradictions_SuppressesContradictoryFinding(t *testing.T) {
	evidenceDir := t.TempDir()
	writeContractFile(t, evidenceDir, contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{{
			Name: "accepted run produces reinforcement proposals",
			Assertions: []contract.ContractAssertion{{
				FileContains: &contract.FileContainsAssertion{
					Path:    "internal/next/reviewdistiller/types.go",
					Pattern: "ModelTier",
				},
			}},
		}},
	})

	findings := []review.Finding{
		{
			Facet:        "architecture_drift",
			Severity:     review.SeverityError,
			File:         "internal/next/reviewdistiller/types.go",
			Description:  "DistillerInputs has a ModelTier field that is not in the spec",
			SuggestedFix: "Remove the ModelTier field from DistillerInputs entirely.",
		},
		{
			Facet:        "code_quality",
			Severity:     review.SeverityError,
			File:         "cmd/gromit-next/review_distill.go",
			Description:  "stubLLMCompleter is placeholder code",
			SuggestedFix: "Replace with real adapter",
		},
	}

	filtered, suppressed := filterContractContradictions(findings, evidenceDir)
	if suppressed != 1 {
		t.Errorf("expected 1 suppressed, got %d", suppressed)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 remaining finding, got %d", len(filtered))
	}
	if filtered[0].File != "cmd/gromit-next/review_distill.go" {
		t.Errorf("wrong finding kept: %s", filtered[0].File)
	}
}

func TestFilterContractContradictions_NoSuppressionWithoutRemoval(t *testing.T) {
	evidenceDir := t.TempDir()
	writeContractFile(t, evidenceDir, contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{{
			Name: "test",
			Assertions: []contract.ContractAssertion{{
				FileContains: &contract.FileContainsAssertion{
					Path:    "types.go",
					Pattern: "Foo",
				},
			}},
		}},
	})

	findings := []review.Finding{{
		Facet:        "code_quality",
		Severity:     review.SeverityError,
		File:         "types.go",
		Description:  "Foo is poorly named",
		SuggestedFix: "Rename Foo to something descriptive",
	}}

	filtered, suppressed := filterContractContradictions(findings, evidenceDir)
	if suppressed != 0 {
		t.Errorf("expected 0 suppressed, got %d", suppressed)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 finding, got %d", len(filtered))
	}
}

func TestFilterContractContradictions_AllSuppressedReturnsEmpty(t *testing.T) {
	evidenceDir := t.TempDir()
	writeContractFile(t, evidenceDir, contract.ScenarioContract{
		Scenarios: []contract.ScenarioAssertions{{
			Name: "test",
			Assertions: []contract.ContractAssertion{{
				FileContains: &contract.FileContainsAssertion{
					Path:    "types.go",
					Pattern: "ModelTier",
				},
			}},
		}},
	})

	findings := []review.Finding{{
		Facet:        "architecture_drift",
		Severity:     review.SeverityError,
		File:         "types.go",
		Description:  "ModelTier should not be here",
		SuggestedFix: "Remove ModelTier from the struct",
	}}

	filtered, suppressed := filterContractContradictions(findings, evidenceDir)
	if suppressed != 1 {
		t.Errorf("expected 1 suppressed, got %d", suppressed)
	}
	if len(filtered) != 0 {
		t.Errorf("expected 0 findings, got %d", len(filtered))
	}
}

func TestFilterContractContradictions_NoContractFileReturnsAll(t *testing.T) {
	findings := []review.Finding{{
		Facet:       "test",
		Severity:    review.SeverityError,
		File:        "foo.go",
		Description: "something",
	}}

	filtered, suppressed := filterContractContradictions(findings, t.TempDir())
	if suppressed != 0 {
		t.Errorf("expected 0 suppressed, got %d", suppressed)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 finding, got %d", len(filtered))
	}
}

func TestFilterContractContradictions_EmptyEvidenceDirReturnsAll(t *testing.T) {
	findings := []review.Finding{{
		Facet:       "test",
		Severity:    review.SeverityError,
		File:        "foo.go",
		Description: "something",
	}}

	filtered, suppressed := filterContractContradictions(findings, "")
	if suppressed != 0 {
		t.Errorf("expected 0 suppressed, got %d", suppressed)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 finding, got %d", len(filtered))
	}
}

func TestPathMatches_ExactMatch(t *testing.T) {
	if !pathMatches("internal/next/types.go", "internal/next/types.go") {
		t.Error("exact match should return true")
	}
}

func TestPathMatches_SuffixMatch(t *testing.T) {
	if !pathMatches("reviewdistiller/types.go", "internal/next/reviewdistiller/types.go") {
		t.Error("suffix match should return true")
	}
}

func TestPathMatches_BaseNameAloneDoesNotMatch(t *testing.T) {
	if pathMatches("types.go", "internal/next/reviewdistiller/types.go") {
		t.Error("basename-only should NOT match — too many files share common names")
	}
}

func TestPathMatches_NoMatch(t *testing.T) {
	if pathMatches("other.go", "internal/next/types.go") {
		t.Error("different files should not match")
	}
}

func TestIsContradicted_DeleteKeyword(t *testing.T) {
	f := review.Finding{
		File:         "types.go",
		Description:  "Foo field is wrong",
		SuggestedFix: "Delete the Foo field entirely",
	}
	required := []contractRequirement{{path: "types.go", pattern: "Foo"}}
	if !isContradicted(f, required) {
		t.Error("finding with 'delete' + matching pattern should be contradicted")
	}
}

func TestIsContradicted_DropKeyword(t *testing.T) {
	f := review.Finding{
		File:         "types.go",
		Description:  "Foo field is wrong",
		SuggestedFix: "Drop the Foo field",
	}
	required := []contractRequirement{{path: "types.go", pattern: "Foo"}}
	if !isContradicted(f, required) {
		t.Error("finding with 'drop' + matching pattern should be contradicted")
	}
}

func TestIsContradicted_NoSuggestedFix(t *testing.T) {
	f := review.Finding{
		File:        "types.go",
		Description: "ModelTier is wrong",
	}
	required := []contractRequirement{{path: "types.go", pattern: "ModelTier"}}
	if isContradicted(f, required) {
		t.Error("finding with no suggested fix should not be contradicted")
	}
}

func TestIsContradicted_PatternInDescriptionNotFix(t *testing.T) {
	f := review.Finding{
		File:         "types.go",
		Description:  "ModelTier is poorly typed",
		SuggestedFix: "Remove the field from DistillerInputs",
	}
	required := []contractRequirement{{path: "types.go", pattern: "ModelTier"}}
	if !isContradicted(f, required) {
		t.Error("pattern in description + removal in fix should be contradicted")
	}
}

func writeContractFile(t *testing.T, dir string, sc contract.ScenarioContract) {
	t.Helper()
	data, err := yaml.Marshal(sc)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), data, 0644); err != nil {
		t.Fatalf("write contract: %v", err)
	}
}
