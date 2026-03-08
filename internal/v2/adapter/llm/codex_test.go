package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexAdapter_Invoke_BuildsCorrectArgs(t *testing.T) {
	tmpDir := t.TempDir()
	fake := filepath.Join(tmpDir, "codex")
	// Script prints the args it received, then outputs valid JSONL.
	script := `#!/usr/bin/env sh
echo "ARGS: $@" >&2
cat > /dev/null
cat <<'EOF'
{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5,"total_cost_usd":0.01}}
EOF
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	provider := NewCodexAdapter(fake, []string{"--some-flag"}, 5*time.Second)
	resp, err := provider.Invoke(context.Background(), InvokeRequest{
		Prompt: "test prompt",
		Model:  "o3",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Invoke returned nil response")
	}

	// Verify the adapter builds correct args by checking the response parses correctly.
	// The real arg validation is done via the internal buildExecCommandArgs which we test below.
	if resp.Output != "ok" {
		t.Fatalf("output = %q, want %q", resp.Output, "ok")
	}
}

func TestCodexAdapter_Invoke_BuildsCorrectArgs_Unit(t *testing.T) {
	a := &codexAdapter{
		binary:          "codex",
		flags:           []string{"--some-flag"},
		timeout:         5 * time.Second,
		tierToReasoning: map[string]string{},
	}

	args := a.buildExecCommandArgs("o3", "")
	joined := strings.Join(args, " ")

	// Must have exec as first arg.
	if args[0] != "exec" {
		t.Fatalf("first arg = %q, want %q", args[0], "exec")
	}
	// Must include --json.
	if !strings.Contains(joined, "--json") {
		t.Fatalf("args missing --json: %s", joined)
	}
	// Must include --model.
	if !strings.Contains(joined, "--model o3") {
		t.Fatalf("args missing --model: %s", joined)
	}
	// Must include --skip-git-repo-check.
	if !strings.Contains(joined, "--skip-git-repo-check") {
		t.Fatalf("args missing --skip-git-repo-check: %s", joined)
	}
	// Must include --full-auto.
	if !strings.Contains(joined, "--full-auto") {
		t.Fatalf("args missing --full-auto: %s", joined)
	}
	// Must NOT include -p (that's a claude flag).
	for _, arg := range args {
		if arg == "-p" {
			t.Fatalf("args should not contain -p (claude print flag): %s", joined)
		}
	}
	// Must NOT include --output-format (that's a claude flag).
	if strings.Contains(joined, "--output-format") {
		t.Fatalf("args should not contain --output-format: %s", joined)
	}
	// Must end with "-" (stdin marker).
	if args[len(args)-1] != "-" {
		t.Fatalf("last arg = %q, want %q", args[len(args)-1], "-")
	}
}

func TestCodexAdapter_Invoke_ParsesOutput(t *testing.T) {
	tmpDir := t.TempDir()
	fake := filepath.Join(tmpDir, "codex")
	script := `#!/usr/bin/env sh
cat > /dev/null
cat <<'EOF'
{"type":"item.completed","item":{"type":"agent_message","text":"hello from codex"}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50,"total_cost_usd":0.42}}
EOF
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	provider := NewCodexAdapter(fake, nil, 5*time.Second)
	resp, err := provider.Invoke(context.Background(), InvokeRequest{Prompt: "test", Model: "o3"})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Invoke returned nil response")
	}

	if resp.Output != "hello from codex" {
		t.Fatalf("output = %q, want %q", resp.Output, "hello from codex")
	}
	if resp.CostUSD != 0.42 {
		t.Fatalf("cost = %f, want %f", resp.CostUSD, 0.42)
	}
	if resp.Tokens != 150 {
		t.Fatalf("tokens = %d, want %d", resp.Tokens, 150)
	}
	if resp.InputTokens != 100 {
		t.Fatalf("input tokens = %d, want %d", resp.InputTokens, 100)
	}
	if resp.OutputTokens != 50 {
		t.Fatalf("output tokens = %d, want %d", resp.OutputTokens, 50)
	}
	if !resp.Success {
		t.Fatal("expected Success to be true")
	}
	if resp.Duration <= 0 {
		t.Fatalf("duration = %v, want positive", resp.Duration)
	}
}

func TestCodexAdapter_Invoke_HandlesFailure(t *testing.T) {
	tmpDir := t.TempDir()
	fake := filepath.Join(tmpDir, "codex")
	script := `#!/usr/bin/env sh
cat > /dev/null
echo "something went wrong" >&2
exit 1
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	provider := NewCodexAdapter(fake, nil, 5*time.Second)
	resp, err := provider.Invoke(context.Background(), InvokeRequest{Prompt: "test", Model: "o3"})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Invoke returned nil response")
	}

	if resp.Success {
		t.Fatal("expected Success to be false for exit code 1")
	}
	if !strings.Contains(resp.Output, "something went wrong") {
		t.Fatalf("output should contain stderr, got %q", resp.Output)
	}
}

func TestCodexAdapter_Invoke_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	fake := filepath.Join(tmpDir, "codex")
	script := `#!/usr/bin/env sh
cat > /dev/null
sleep 30
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	provider := NewCodexAdapter(fake, nil, 500*time.Millisecond)
	_, err := provider.Invoke(context.Background(), InvokeRequest{Prompt: "test", Model: "o3"})
	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestCodexAdapter_Invoke_BypassApprovalsExcludesFullAuto(t *testing.T) {
	a := &codexAdapter{
		binary:          "codex",
		flags:           []string{"--dangerously-bypass-approvals-and-sandbox"},
		timeout:         5 * time.Second,
		tierToReasoning: map[string]string{},
	}

	args := a.buildExecCommandArgs("o3", "")
	joined := strings.Join(args, " ")

	// --full-auto should be omitted when --dangerously-bypass-approvals-and-sandbox is present.
	if strings.Contains(joined, "--full-auto") {
		t.Fatalf("args should NOT contain --full-auto when bypass flag is present: %s", joined)
	}
	// But the bypass flag itself should be there.
	if !strings.Contains(joined, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("args missing bypass flag: %s", joined)
	}
}

func TestCodexAdapter_StreamInvoke_StreamsOutput(t *testing.T) {
	tmpDir := t.TempDir()
	fake := filepath.Join(tmpDir, "codex")
	script := `#!/usr/bin/env sh
cat > /dev/null
cat <<'EOF'
{"type":"item.completed","item":{"type":"agent_message","text":"streamed output"}}
{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10,"total_cost_usd":0.15}}
EOF
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	provider := NewCodexAdapter(fake, nil, 5*time.Second)
	var streamed strings.Builder
	resp, err := provider.StreamInvoke(context.Background(), StreamInvokeRequest{
		Prompt: "streaming prompt",
		Model:  "o3",
		Output: &streamed,
	})
	if err != nil {
		t.Fatalf("StreamInvoke returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("StreamInvoke returned nil response")
	}

	if !strings.Contains(streamed.String(), "streamed output") {
		t.Fatalf("stream output missing agent text: %q", streamed.String())
	}
	if resp.Output != "streamed output" {
		t.Fatalf("output = %q, want %q", resp.Output, "streamed output")
	}
	if resp.Tokens != 30 {
		t.Fatalf("tokens = %d, want %d", resp.Tokens, 30)
	}
	if resp.InputTokens != 20 {
		t.Fatalf("input tokens = %d, want %d", resp.InputTokens, 20)
	}
	if resp.OutputTokens != 10 {
		t.Fatalf("output tokens = %d, want %d", resp.OutputTokens, 10)
	}
	if resp.CostUSD != 0.15 {
		t.Fatalf("cost = %f, want %f", resp.CostUSD, 0.15)
	}
	if !resp.Success {
		t.Fatal("expected Success to be true")
	}
}

func TestCodexAdapter_Invoke_ModelRequired(t *testing.T) {
	provider := NewCodexAdapter("codex", nil, 5*time.Second)
	_, err := provider.Invoke(context.Background(), InvokeRequest{Prompt: "test"})
	if err == nil {
		t.Fatal("expected error when model is empty")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("expected 'model is required' error, got %v", err)
	}
}

func TestCodexAdapter_ReasoningEffort(t *testing.T) {
	a := &codexAdapter{
		binary:  "codex",
		flags:   nil,
		timeout: 5 * time.Second,
		tierToReasoning: map[string]string{
			"high": "high",
		},
	}

	args := a.buildExecCommandArgs("o3", "high")
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "-c model_reasoning_effort=high") {
		t.Fatalf("args missing reasoning effort: %s", joined)
	}
}
