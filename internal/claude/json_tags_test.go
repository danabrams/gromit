package claude

import (
	"reflect"
	"testing"
)

func TestResultJSONTags(t *testing.T) {
	fields := map[string]string{
		"Success":      "success",
		"Output":       "output",
		"ExitCode":     "exit_code",
		"Duration":     "duration",
		"Model":        "model",
		"CostUSD":      "cost_usd",
		"InputTokens":  "input_tokens",
		"OutputTokens": "output_tokens",
	}

	resultType := reflect.TypeOf(Result{})

	for fieldName, wantTag := range fields {
		field, ok := resultType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("missing Result field %s", fieldName)
		}

		if got := field.Tag.Get("json"); got != wantTag {
			t.Errorf("Result.%s json tag = %q, want %q", fieldName, got, wantTag)
		}
	}
}
