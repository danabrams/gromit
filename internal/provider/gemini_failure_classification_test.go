package provider

import "testing"

func TestClassifyGeminiFailure_Success(t *testing.T) {
	t.Parallel()
	category := classifyGeminiFailure(0, "")
	if category != FailureCategoryNone {
		t.Errorf("expected FailureCategoryNone for exit code 0, got %q", category)
	}
}

func TestClassifyGeminiFailure_AuthError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stderr string
	}{
		{"invalid api key", "error: invalid api key"},
		{"unauthorized", "unauthorized request"},
		{"forbidden", "forbidden: access denied"},
		{"authentication", "authentication failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryAuth {
				t.Errorf("expected FailureCategoryAuth, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_StartupError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stderr string
	}{
		{"failed to start", "failed to start gemini"},
		{"initialization error", "initialization error"},
		{"startup failed", "startup failed: no api key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryStartupError {
				t.Errorf("expected FailureCategoryStartupError, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_TransportError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stderr string
	}{
		{"connection reset", "connection reset by peer"},
		{"timeout", "request timeout"},
		{"service unavailable", "service unavailable"},
		{"broken pipe", "broken pipe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryTransportDisconnect {
				t.Errorf("expected FailureCategoryTransportDisconnect, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_RateLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stderr string
	}{
		{"rate limit", "rate limit exceeded"},
		{"quota exceeded", "quota exceeded"},
		{"too many requests", "too many requests"},
		{"429 status", "HTTP 429"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			category := classifyGeminiFailure(1, tt.stderr)
			if category != FailureCategoryRateLimited {
				t.Errorf("expected FailureCategoryRateLimited, got %q", category)
			}
		})
	}
}

func TestClassifyGeminiFailure_Other(t *testing.T) {
	t.Parallel()
	category := classifyGeminiFailure(1, "unknown error")
	if category != FailureCategoryOther {
		t.Errorf("expected FailureCategoryOther, got %q", category)
	}
}
