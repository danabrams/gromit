package wiring_test

import (
    "reflect"
    "strings"
    "testing"

    wiring "github.com/danabrams/gromit/internal/pipeline/qualitygate/wiring"
)

func TestExtractSymbolsFromDiff_DetectsExportedFunction(t *testing.T) {
    diff := strings.Join([]string{
        "diff --git a/foo.go b/foo.go",
        "new file mode 100644",
        "index 0000000..1111111",
        "--- /dev/null",
        "+++ b/foo.go",
        "@@ -0,0 +1,3 @@",
        "+package main",
        "+",
        "+func Foo() {}",
    }, "\n")

    got := wiring.ExtractSymbolsFromDiff(diff)
    want := []wiring.Symbol{{Name: "Foo", File: "foo.go", Line: 3}}

    if !reflect.DeepEqual(got, want) {
        t.Fatalf("ExtractSymbolsFromDiff() = %v, want %v", got, want)
    }
}

func TestExtractSymbolsFromDiff_FindsTypesMethodsAndFields(t *testing.T) {
    diff := strings.Join([]string{
        "diff --git a/foo.go b/foo.go",
        "new file mode 100644",
        "index 0000000..1111111",
        "--- /dev/null",
        "+++ b/foo.go",
        "@@ -0,0 +1,9 @@",
        "+package foo",
        "+",
        "+type ExportedType struct {",
        "+    ExportedField string",
        "+    unexportedField int",
        "+}",
        "+",
        "+func (t ExportedType) ExportedMethod() {}",
    }, "\n")

    got := wiring.ExtractSymbolsFromDiff(diff)
    want := []wiring.Symbol{
        {Name: "ExportedType", File: "foo.go", Line: 3},
        {Name: "ExportedField", File: "foo.go", Line: 4},
        {Name: "ExportedMethod", File: "foo.go", Line: 8},
    }

    if !reflect.DeepEqual(got, want) {
        t.Fatalf("ExtractSymbolsFromDiff() = %v, want %v", got, want)
    }
}

func TestExtractSymbolsFromDiff_SkipsTestFiles(t *testing.T) {
    diff := strings.Join([]string{
        "diff --git a/foo_test.go b/foo_test.go",
        "new file mode 100644",
        "index 0000000..1111111",
        "--- /dev/null",
        "+++ b/foo_test.go",
        "@@ -0,0 +1,4 @@",
        "+package foo",
        "+",
        "+func TestHelper() {}",
    }, "\n")

    if got := wiring.ExtractSymbolsFromDiff(diff); len(got) != 0 {
        t.Fatalf("ExtractSymbolsFromDiff() = %v, want no symbols", got)
    }
}
