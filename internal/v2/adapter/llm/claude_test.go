package llm

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestClaudeAdapter_InvokeParsesJSON(t *testing.T) {
    t.Helper()

    tmpDir := t.TempDir()
    fake := filepath.Join(tmpDir, "claude")
    script := `#!/usr/bin/env sh
cat > /dev/null
cat <<'EOF'
{"success": true, "result": "hello from json", "total_cost_usd": 0.42, "input_tokens": 4, "output_tokens": 6}
EOF
`
    if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
        t.Fatalf("write fake claude: %v", err)
    }

    provider := NewClaudeAdapter(fake, nil, 5*time.Second)
    resp, err := provider.Invoke(context.Background(), InvokeRequest{Prompt: "print me", Model: "opus"})
    if err != nil {
        t.Fatalf("Invoke returned error: %v", err)
    }
    if resp == nil {
        t.Fatal("Invoke returned nil response")
    }

    if resp.Output != "hello from json" {
        t.Fatalf("output = %q, want %q", resp.Output, "hello from json")
    }
    if resp.CostUSD != 0.42 {
        t.Fatalf("cost = %f, want %f", resp.CostUSD, 0.42)
    }
    if resp.Tokens != 10 {
        t.Fatalf("tokens = %d, want %d", resp.Tokens, 10)
    }
    if resp.Duration <= 0 {
        t.Fatalf("duration = %v, want positive", resp.Duration)
    }
}
