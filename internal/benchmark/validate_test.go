package benchmark

import (
	"errors"
	stdstrings "strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

type fakeBeadLookup struct {
	showFn func(id string) (*bead.Bead, error)
}

func (f fakeBeadLookup) Show(id string) (*bead.Bead, error) {
	if f.showFn == nil {
		return nil, errors.New("missing showFn")
	}
	return f.showFn(id)
}

func TestValidateSelectedCohort_EnforcesMinimumSize(t *testing.T) {
	_, err := ValidateSelectedCohort(nil, []string{"gromit-1"}, 2, true)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want minimum size error")
	}
	if !stdstrings.Contains(err.Error(), "selected cohort size 1 is below minimum 2") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want minimum size message", err.Error())
	}
}

func TestValidateSelectedCohort_ReturnsErrorWhenBeadDoesNotExist(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			if id != "gromit-missing" {
				t.Fatalf("Show() id = %q, want %q", id, "gromit-missing")
			}
			return nil, errors.New("not found")
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-missing"}, 1, true)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want missing bead error")
	}
	if !stdstrings.Contains(err.Error(), "gromit-missing") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want missing bead id", err.Error())
	}
}

func TestValidateSelectedCohort_ReturnsErrorWhenLookupReturnsNilBead(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			return nil, nil
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-1"}, 1, false)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want nil-bead error")
	}
	if !stdstrings.Contains(err.Error(), "lookup returned nil bead") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want nil-bead message", err.Error())
	}
}

func TestValidateSelectedCohort_ReturnsErrorWhenBeadIsClosed(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			return &bead.Bead{ID: id, Status: "closed"}, nil
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-1"}, 1, true)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want closed bead error")
	}
	if !stdstrings.Contains(err.Error(), "must be open") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want open-state error", err.Error())
	}
}

func TestComplexityTier_DefaultsToMediumWhenUnlabeled(t *testing.T) {
	got := complexityTier([]string{"spec:runner"})
	if got != "medium" {
		t.Fatalf("complexityTier() = %q, want %q", got, "medium")
	}
}

func TestValidateSelectedCohort_ReturnsErrorWhenTierCoverageMissing(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			switch id {
			case "gromit-1":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:low"}}, nil
			case "gromit-2", "gromit-3":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:high"}}, nil
			default:
				return nil, errors.New("unexpected id")
			}
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-1", "gromit-2", "gromit-3"}, 3, true)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want missing-tier error")
	}
	if !stdstrings.Contains(err.Error(), "missing complexity tiers: medium") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want missing medium tier message", err.Error())
	}
}

func TestValidateSelectedCohort_ReturnsErrorForUnsupportedComplexityLabel(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			switch id {
			case "gromit-1":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:low"}}, nil
			case "gromit-2":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:urgent"}}, nil
			case "gromit-3":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:high"}}, nil
			default:
				return nil, errors.New("unexpected id")
			}
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-1", "gromit-2", "gromit-3"}, 3, true)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want unsupported complexity error")
	}
	if !stdstrings.Contains(err.Error(), "unsupported complexity label") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want unsupported complexity message", err.Error())
	}
}

func TestValidateSelectedCohort_RejectsSizeGreaterThanFive(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			switch id {
			case "gromit-1":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:low"}}, nil
			case "gromit-2":
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:high"}}, nil
			default:
				return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:medium"}}, nil
			}
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5", "gromit-6"}, 5, true)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want fixed-size cohort error")
	}
}

func TestValidateSelectedCohort_SkipsTierCoverageWhenDisabled(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			return &bead.Bead{ID: id, Status: "open", Labels: []string{"complexity:urgent"}}, nil
		},
	}

	selected, err := ValidateSelectedCohort(lookup, []string{"gromit-1"}, 1, false)
	if err != nil {
		t.Fatalf("ValidateSelectedCohort() error = %v", err)
	}
	if len(selected) != 1 || selected[0] != "gromit-1" {
		t.Fatalf("selected = %v, want [gromit-1]", selected)
	}
}
