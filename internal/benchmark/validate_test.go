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
	_, err := ValidateSelectedCohort(nil, []string{"gromit-1"}, 2)
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

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-missing"}, 1)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want missing bead error")
	}
	if !stdstrings.Contains(err.Error(), "gromit-missing") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want missing bead id", err.Error())
	}
}

func TestValidateSelectedCohort_ReturnsErrorWhenBeadIsClosed(t *testing.T) {
	lookup := fakeBeadLookup{
		showFn: func(id string) (*bead.Bead, error) {
			return &bead.Bead{ID: id, Status: "closed"}, nil
		},
	}

	_, err := ValidateSelectedCohort(lookup, []string{"gromit-1"}, 1)
	if err == nil {
		t.Fatal("ValidateSelectedCohort() error = nil, want closed bead error")
	}
	if !stdstrings.Contains(err.Error(), "must be open") {
		t.Fatalf("ValidateSelectedCohort() error = %q, want open-state error", err.Error())
	}
}
