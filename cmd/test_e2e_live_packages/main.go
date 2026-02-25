package main

import (
    "flag"
    "fmt"
    "os"

    "github.com/danabrams/gromit/internal/testpkg"
)

func main() {
    root := flag.String("root", ".", "project root")
    tag := flag.String("tag", "e2e_live", "build tag to look for")
    flag.Parse()

    pkgs, err := testpkg.FindTaggedPackages(*root, *tag)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to list packages: %v\n", err)
        os.Exit(1)
    }

    for _, pkg := range pkgs {
        fmt.Println(pkg)
    }
}
