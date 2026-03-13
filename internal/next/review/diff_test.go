package review

import "testing"

func TestDiffProvider_Interface(t *testing.T) {
	var dp DiffProvider = &fakeDiffProvider{diff: "some diff"}
	diff, err := dp.Diff("main")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff != "some diff" {
		t.Errorf("diff = %q, want %q", diff, "some diff")
	}
}

type fakeDiffProvider struct {
	diff string
	err  error
}

func (f *fakeDiffProvider) Diff(baseBranch string) (string, error) {
	return f.diff, f.err
}
