package provider

import "testing"

func TestClassifyGeminiCLIError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		stderr      string
		wantCategory string
		wantRetryable bool
	}{
		{
			name:        "setup failure",
			stderr:      "zsh:2: command not found: gemini",
			wantCategory: "setup/binary-missing",
			wantRetryable: false,
		},
		{
			name:        "model invalid",
			stderr:      "ModelNotFoundError: 404 NOT_FOUND",
			wantCategory: "model-invalid",
			wantRetryable: false,
		},
		{
			name:        "permission denied",
			stderr:      "ls: cannot open directory: Permission denied",
			wantCategory: "permission-denied",
			wantRetryable: false,
		},
		{
			name:        "fallback case",
			stderr:      "unexpected panic",
			wantCategory: "fallback",
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyGeminiCLIError(tt.stderr)
			if got.Category != tt.wantCategory {
				t.Fatalf("category = %q, want %q", got.Category, tt.wantCategory)
			}
			if got.Retryable != tt.wantRetryable {
				t.Fatalf("retryable = %t, want %t", got.Retryable, tt.wantRetryable)
			}
		})
	}
}
