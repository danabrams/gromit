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
