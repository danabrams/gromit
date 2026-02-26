//go:build contract

package contracts

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/tracker"
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

func TestBDAdapterContractListFiltersByLabels(t *testing.T) {
	env := setupTestEnv(t)
	adapter := newBDAdapterWithEnv(t, env)

	primary := createFakeBead(t, env, "Tracker adapter label", "--label", "contract-reader")
	createFakeBead(t, env, "Tracker adapter other", "--label", "other")

	items, err := adapter.List(context.Background(), tracker.Query{
		Filter: tracker.Filter{
			Labels: []string{"contract-reader"},
		},
	})
	if err != nil {
		t.Fatalf("adapter list failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != primary.ID {
		t.Fatalf("list item ID = %q, want %q", items[0].ID, primary.ID)
	}

	requireBDListCall(t, env, "bd list --json --status open --sort priority --limit 0")
}

func TestBDAdapterContractCreateCloseSync(t *testing.T) {
	env := setupTestEnv(t)
	adapter := newBDAdapterWithEnv(t, env)

	req := tracker.CreateRequest{
		Title:       "Adapter contract create",
		Description: "Create via BDAdapter",
		Metadata: map[string]string{
			"priority": "1",
			"labels":   "[\"bdadapter\"]",
		},
	}

	created, err := adapter.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("adapter create failed: %v", err)
	}
	if created == nil {
		t.Fatal("expected created item, got nil")
	}
	if got := created.Metadata["labels"]; got != "[\"bdadapter\"]" {
		t.Fatalf("created metadata labels = %q, want %q", got, "[\"bdadapter\"]")
	}

	ready, err := adapter.Ready(context.Background())
	if err != nil {
		t.Fatalf("ready after create failed: %v", err)
	}
	if ready == nil || ready.ID != created.ID {
		t.Fatalf("ready after create = %v, want ID %q", ready, created.ID)
	}

	if err := adapter.Sync(context.Background()); err != nil {
		t.Fatalf("adapter sync failed: %v", err)
	}
	requireBDCall(t, env, "bd sync")

	if err := adapter.Close(context.Background(), created.ID); err != nil {
		t.Fatalf("adapter close failed: %v", err)
	}
	requireBDCallContains(t, env, "bd close "+created.ID)

	afterClose, err := adapter.Ready(context.Background())
	if err != nil {
		t.Fatalf("ready after close failed: %v", err)
	}
	if afterClose != nil {
		t.Fatalf("expected no ready beads after close, got %v", afterClose)
	}
	requireBDCall(t, env, "bd ready --json --limit 3")
}

func newBDAdapterWithEnv(t *testing.T, env *testEnv) *bead.BDAdapter {
	t.Helper()
	applyAdapterEnv(t, env)

	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("new bead client failed: %v", err)
	}
	client.Dir = env.Dir

	return bead.NewBDAdapter(client)
}

func applyAdapterEnv(t *testing.T, env *testEnv) {
	t.Helper()
	t.Setenv("PATH", env.PATH)
	t.Setenv("TEST_DIR", env.Dir)
	t.Setenv("TEST_CALL_LOG", env.CallLog)
}

func createFakeBead(t *testing.T, env *testEnv, args ...string) *bead.Bead {
	t.Helper()

	cmdArgs := append([]string{"create"}, args...)
	if !containsString(cmdArgs, "--json") {
		cmdArgs = append(cmdArgs, "--json")
	}

	cmd := exec.Command(filepath.Join(fakesDir, "bd"), cmdArgs...)
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fake bd create failed: %v\nOutput: %s", err, output)
	}

	var created bead.Bead
	if err := json.Unmarshal(output, &created); err != nil {
		t.Fatalf("unmarshal fake bd create output: %v\nOutput: %s", err, output)
	}

	return &created
}

func containsString(slice []string, target string) bool {
	for _, value := range slice {
		if value == target {
			return true
		}
	}
	return false
}

func requireBDCall(t *testing.T, env *testEnv, expected string) {
	t.Helper()

	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	for _, call := range calls {
		if call == expected {
			return
		}
	}

	t.Fatalf("expected bd call %q not found; calls: %v", expected, calls)
}

func requireBDListCall(t *testing.T, env *testEnv, expected string) {
	t.Helper()
	requireBDCall(t, env, expected)
}

func requireBDCallContains(t *testing.T, env *testEnv, substring string) {
	t.Helper()

	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	for _, call := range calls {
		if strings.Contains(call, substring) {
			return
		}
	}

	t.Fatalf("expected bd calls to contain %q; calls: %v", substring, calls)
}
