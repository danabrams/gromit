package prompt

import "testing"

func TestContextExperimentAndVariantIDsHoldValues(t *testing.T) {
	ctx := &Context{
		ExperimentID: "exp-123",
		VariantID:    "var-456",
	}

	if ctx.ExperimentID != "exp-123" {
		t.Fatalf("ExperimentID = %q, want %q", ctx.ExperimentID, "exp-123")
	}
	if ctx.VariantID != "var-456" {
		t.Fatalf("VariantID = %q, want %q", ctx.VariantID, "var-456")
	}
}

func TestReviewContextNormalizeNilFields(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ReviewContext
		want []string
	}{
		{
			name: "nil validation commands",
			ctx:  &ReviewContext{},
			want: []string{},
		},
		{
			name: "existing validation commands",
			ctx: &ReviewContext{
				ValidationCommands: []string{"go test ./..."},
			},
			want: []string{"go test ./..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			ctx.normalizeNilFields()
			if len(ctx.ValidationCommands) != len(tt.want) {
				t.Fatalf("len(ValidationCommands) = %d, want %d", len(ctx.ValidationCommands), len(tt.want))
			}
			for i := range tt.want {
				if ctx.ValidationCommands[i] != tt.want[i] {
					t.Fatalf("ValidationCommands[%d] = %q, want %q", i, ctx.ValidationCommands[i], tt.want[i])
				}
			}
		})
	}
}

func TestThoroughReviewContextNormalizeNilFields(t *testing.T) {
	ctx := &ThoroughReviewContext{}
	ctx.normalizeNilFields()
	if ctx.CompletedBeads == nil {
		t.Fatalf("CompletedBeads nil after normalizeNilFields")
	}
}

func TestTDDRedContextNormalizeNilFields(t *testing.T) {
	ctx := &TDDRedContext{}
	ctx.normalizeNilFields()
	if ctx.TestFileContents == nil {
		t.Fatalf("TestFileContents nil after normalizeNilFields")
	}
}
