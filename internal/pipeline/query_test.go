package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListUnplannedSpecs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		specs []string
		plans []string
		want  []string
	}{
		{
			name:  "specs without plans",
			specs: []string{"user-auth", "dark-mode"},
			plans: []string{"dark-mode"},
			want:  []string{"user-auth"},
		},
		{
			name: "empty directories",
			want: []string{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			specsDir := filepath.Join(tmp, "specs")
			plansDir := filepath.Join(tmp, "plans")
			if err := os.MkdirAll(specsDir, 0755); err != nil {
				t.Fatalf("MkdirAll specs: %v", err)
			}
			if err := os.MkdirAll(plansDir, 0755); err != nil {
				t.Fatalf("MkdirAll plans: %v", err)
			}

			for _, spec := range tc.specs {
				path := filepath.Join(specsDir, spec+".md")
				if err := os.WriteFile(path, []byte("# "+spec+" spec"), 0644); err != nil {
					t.Fatalf("Write spec: %v", err)
				}
			}

			for _, plan := range tc.plans {
				path := filepath.Join(plansDir, plan+".md")
				if err := os.WriteFile(path, []byte("# "+plan+" plan"), 0644); err != nil {
					t.Fatalf("Write plan: %v", err)
				}
			}

			got, err := ListUnplannedSpecs(specsDir, plansDir)
			if err != nil {
				t.Fatalf("ListUnplannedSpecs error: %v", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ListUnplannedSpecs = %#v, want %#v", got, tc.want)
			}
		})
	}
}
