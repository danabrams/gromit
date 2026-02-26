//go:build contract

package contracts

import (
	"context"
	"testing"
)

func TestBDAdapterContractReady(t *testing.T) {
	env := setupTestEnv(t)
	adapter := newBDAdapterWithEnv(t, env)

	created := createFakeBead(t, env, "Contract adapter ready", "--priority", "1")

	item, err := adapter.Ready(context.Background())
	if err != nil {
		t.Fatalf("adapter ready failed: %v", err)
	}
	if item == nil {
		t.Fatal("expected ready item, got nil")
	}
	if item.ID != created.ID {
		t.Fatalf("ready ID = %q, want %q", item.ID, created.ID)
	}

	requireBDCall(t, env, "bd ready --json --limit 3")
}
