package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestListUndecomposedPlans(t *testing.T) {
	t.Parallel()

	t.Run("returns undecomposed plans", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		plansDir := filepath.Join(tmp, "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("MkdirAll plans: %v", err)
		}

		writePlan := func(name, content string) {
			path := filepath.Join(plansDir, name+".md")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("write plan %s: %v", name, err)
			}
		}

		writePlan("user-auth", "---\ndecomposed: false\n---\n# Plan")
		writePlan("dark-mode", "---\ndecomposed: true\n---\n# Plan")
		writePlan("api", "# Plan with no frontmatter")

		got, err := ListUndecomposedPlans(plansDir)
		if err != nil {
			t.Fatalf("ListUndecomposedPlans error: %v", err)
		}

		want := []string{"api", "user-auth"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListUndecomposedPlans = %#v, want %#v", got, want)
		}
	})

	t.Run("malformed frontmatter returns error", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		plansDir := filepath.Join(tmp, "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("MkdirAll plans: %v", err)
		}

		path := filepath.Join(plansDir, "bogus.md")
		if err := os.WriteFile(path, []byte("---\nnot yaml: [\n---\n# Plan"), 0644); err != nil {
			t.Fatalf("write bogus plan: %v", err)
		}

		if _, err := ListUndecomposedPlans(plansDir); err == nil {
			t.Fatalf("expected error for malformed frontmatter")
		} else if !strings.Contains(err.Error(), "frontmatter") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
