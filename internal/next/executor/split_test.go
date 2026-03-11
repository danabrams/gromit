package executor

import (
	"testing"
)

func TestNeedsSplit_ThreeDistinctPackages(t *testing.T) {
	changedFiles := []string{"pkg/a/x.go", "pkg/b/y.go", "pkg/c/z.go"}
	if !NeedsSplit(changedFiles, []string{"pkg/a/"}) {
		t.Fatal("3+ distinct parent directories should trigger split")
	}
}

func TestNeedsSplit_AreaExceeded(t *testing.T) {
	changed := []string{"a/1.go", "a/2.go", "b/1.go", "b/2.go", "c/1.go"}
	expected := []string{"a/"}
	if !NeedsSplit(changed, expected) {
		t.Fatal("files exceeding 2x expected area should trigger split")
	}
}

func TestNeedsSplit_SmallChange(t *testing.T) {
	if NeedsSplit([]string{"a/x.go"}, []string{"a/"}) {
		t.Fatal("single package should not trigger split")
	}
}

func TestNeedsSplit_TwoPackagesNoSplit(t *testing.T) {
	changed := []string{"a/x.go", "b/y.go"}
	if NeedsSplit(changed, []string{"a/", "b/"}) {
		t.Fatal("2 packages within expected area should not trigger split")
	}
}
