package main

import (
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
)

func mockBeadClientEmptyList() *bead.Client {
	return &bead.Client{
		RunFn: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "list" {
				return "[]", nil
			}
			return "", fmt.Errorf("unexpected bd args: %v", args)
		},
	}
}
