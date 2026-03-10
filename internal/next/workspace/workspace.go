package workspace

import (
	"os"
	"path/filepath"
)

type Root string

func (r Root) ProjectsDir() string {
	return filepath.Join(string(r), "projects")
}

func (r Root) ProjectCell(name string) string {
	return filepath.Join(r.ProjectsDir(), name)
}

type Resolver interface {
	Resolve() (Root, error)
}

type EnvResolver struct{}

func NewEnvResolver() *EnvResolver {
	return &EnvResolver{}
}

func (r *EnvResolver) Resolve() (Root, error) {
	if v := os.Getenv("GROMIT_HOME"); v != "" {
		return Root(v), nil
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return Root(filepath.Join(v, "gromit")), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return Root(filepath.Join(home, ".local", "share", "gromit")), nil
}
