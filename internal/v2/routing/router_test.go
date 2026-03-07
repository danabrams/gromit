package routing_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/v2/routing"
)

func TestRouterSelectReturnsErrorWhenNoProviders(t *testing.T) {
	r := routing.NewRouter(routing.RouterConfig{})
	_, _, err := r.Select("build", "low")
	if err == nil {
		t.Fatal("expected error when no providers configured, got nil")
	}
}
