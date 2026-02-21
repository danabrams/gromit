package escalation

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestTriage(t *testing.T) {
	validBC := &runtypes.BeadContext{
		BuildPrompt: "build prompt",
		Bead:        &bead.Bead{Description: "non-empty"},
	}

	tests := []struct {
		name string
		inv  *runtypes.InvocationResult
		bc   *runtypes.BeadContext
		want *TriageResult
	}{
		{
			name: "provider transport disconnect",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{FailureCategory: provider.FailureCategoryTransportDisconnect},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "disconnect",
				Detail:      provider.FailureCategoryTransportDisconnect,
				Retryable:   true,
			},
		},
		{
			name: "provider rate limited",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{FailureCategory: provider.FailureCategoryRateLimited},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "rate_limit",
				Detail:      provider.FailureCategoryRateLimited,
				Retryable:   true,
			},
		},
		{
			name: "provider auth",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{FailureCategory: provider.FailureCategoryAuth},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "auth",
				Detail:      provider.FailureCategoryAuth,
				Retryable:   false,
			},
		},
		{
			name: "environment missing tool from stderr",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{Stderr: "exec: foo: executable file not found in $PATH"},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "missing_tool",
				Detail:      "exec: foo: executable file not found in $PATH",
				Retryable:   false,
			},
		},
		{
			name: "environment version mismatch from stderr",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{Stderr: "go: go.mod requires go >= 1.24.0"},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "version_mismatch",
				Detail:      "go: go.mod requires go >= 1.24.0",
				Retryable:   false,
			},
		},
		{
			name: "environment resource exhausted from stderr",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{Stderr: "write /tmp/out: no space left on device"},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "resource_exhausted",
				Detail:      "write /tmp/out: no space left on device",
				Retryable:   false,
			},
		},
		{
			name: "environment permission from stderr",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{Stderr: "open /root/file: permission denied"},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "permission",
				Detail:      "open /root/file: permission denied",
				Retryable:   false,
			},
		},
		{
			name: "environment uses output when stderr empty",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{Output: "exec: make: executable file not found in $PATH"},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "missing_tool",
				Detail:      "exec: make: executable file not found in $PATH",
				Retryable:   false,
			},
		},
		{
			name: "waterfall provider transport before environment",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{
					FailureCategory: provider.FailureCategoryAuth,
					Stderr:          "open /root/file: permission denied",
				},
			},
			bc: validBC,
			want: &TriageResult{
				Layer:       LayerProviderTransport,
				SubCategory: "auth",
				Detail:      provider.FailureCategoryAuth,
				Retryable:   false,
			},
		},
		{
			name: "waterfall environment before orchestration",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{Stderr: "open /root/file: permission denied"},
			},
			bc: &runtypes.BeadContext{
				BuildPrompt: "",
				Bead:        &bead.Bead{Description: ""},
			},
			want: &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: "permission",
				Detail:      "open /root/file: permission denied",
				Retryable:   false,
			},
		},
		{
			name: "orchestration bad prompt",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{},
			},
			bc: &runtypes.BeadContext{
				BuildPrompt: "",
				Bead:        &bead.Bead{Description: "non-empty"},
			},
			want: &TriageResult{
				Layer:       LayerOrchestration,
				SubCategory: "bad_prompt",
				Detail:      "build prompt is empty",
				Retryable:   false,
			},
		},
		{
			name: "orchestration bad bead",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{},
			},
			bc: &runtypes.BeadContext{
				BuildPrompt: "non-empty",
				Bead:        &bead.Bead{Description: ""},
			},
			want: &TriageResult{
				Layer:       LayerOrchestration,
				SubCategory: "bad_bead",
				Detail:      "bead description is empty",
				Retryable:   false,
			},
		},
		{
			name: "nil provider result falls through to code",
			inv:  &runtypes.InvocationResult{},
			bc:   validBC,
			want: &TriageResult{Layer: LayerCode, SubCategory: "default", Detail: "code-level or unknown failure", Retryable: true},
		},
		{
			name: "failure category other falls through to code",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{FailureCategory: provider.FailureCategoryOther},
			},
			bc:   validBC,
			want: &TriageResult{Layer: LayerCode, SubCategory: "default", Detail: "code-level or unknown failure", Retryable: true},
		},
		{
			name: "empty strings fall through to code",
			inv: &runtypes.InvocationResult{
				ProviderResult: &provider.Result{FailureCategory: "", Stderr: "", Output: ""},
			},
			bc:   validBC,
			want: &TriageResult{Layer: LayerCode, SubCategory: "default", Detail: "code-level or unknown failure", Retryable: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Triage(tt.inv, tt.bc)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Triage() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
