package logger

import (
	"errors"
	"testing"
)

func TestReadAllIterationLogsSorted_UsesListRunLogFiles(t *testing.T) {
	original := listRunLogFilesFn
	defer func() {
		listRunLogFilesFn = original
	}()

	want := errors.New("boom")
	listRunLogFilesFn = func(string) ([]string, error) {
		return nil, want
	}

	if _, err := readAllIterationLogsSorted("ignored"); !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}
