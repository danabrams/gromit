package names

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestDescribeStages(t *testing.T) {
	cases := []struct {
		name  string
		stage string
		cfg   *config.Config
		want  string
	}{
		{name: "nil config returns stage", stage: "accept", cfg: nil, want: "accept"},
		{name: "default profile", stage: "build", cfg: &config.Config{}, want: "build:default"},
		{name: "profile prepends stage", stage: "review", cfg: &config.Config{Project: config.ProjectConfig{Profile: "prod"}}, want: "prod:review"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := Describe(tt.stage, tt.cfg); got != tt.want {
				t.Fatalf("Describe(%q) = %q, want %q", tt.stage, got, tt.want)
			}
		})
	}
}
