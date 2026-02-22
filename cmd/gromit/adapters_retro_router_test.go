package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/retro"
)

func TestRetroRouterAdapter_ConformsToProviderRunner(t *testing.T) {
	var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)
}
