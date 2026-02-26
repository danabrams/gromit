package config

import (
	"reflect"
	"testing"
)

func TestProfileForName(t *testing.T) {
	testCases := []struct {
		name         string
		wantCommands []string
		wantFound    bool
	}{
		{
			name:         "go",
			wantCommands: []string{"go test", "go build", "go vet"},
			wantFound:    true,
		},
		{
			name:         "node",
			wantCommands: []string{"npm test", "npm run build"},
			wantFound:    true,
		},
		{
			name:         "python",
			wantCommands: []string{"pytest"},
			wantFound:    true,
		},
		{
			name:      "custom",
			wantFound: true,
		},
		{
			name:      "unknown",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaults, ok := ProfileForName(tc.name)
			if ok != tc.wantFound {
				t.Fatalf("found = %v, want %v", ok, tc.wantFound)
			}
			if !tc.wantFound {
				return
			}
			if !reflect.DeepEqual(defaults.ValidationCommands, tc.wantCommands) {
				t.Fatalf("ValidationCommands = %v, want %v", defaults.ValidationCommands, tc.wantCommands)
			}
		})
	}
}
