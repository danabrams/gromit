// Package projectcell manages per-project storage cells within the workspace.
//
// Each attached project gets its own cell directory containing all derived
// artifacts (architecture.json, source-map.json, etc.). Cells are keyed by
// a stable project identifier derived from the repo URL or local path.
//
// TODO: implement project registration (create cell directory, write project.yaml)
// TODO: implement project listing (enumerate cells in workspace)
// TODO: implement project lookup by repo path or alias
// TODO: implement multi-project isolation (cells must not share mutable state)
package projectcell

// Cell represents a single project's storage directory within the workspace.
type Cell struct {
	// ID is the stable identifier for this project cell.
	ID string

	// Path is the absolute path to the cell directory.
	Path string

	// Project holds the project configuration.
	Project ProjectConfig
}

// ProjectConfig is the in-memory representation of project.yaml.
//
// TODO: implement YAML serialization
// TODO: implement validation (required fields, path existence)
type ProjectConfig struct {
	// Name is the human-readable project name.
	Name string `yaml:"name"`

	// RepoPath is the absolute path to the project's git repository.
	RepoPath string `yaml:"repo_path"`

	// Alias is an optional short name for CLI convenience.
	Alias string `yaml:"alias,omitempty"`
}

// Store manages project cell lifecycle within a workspace.
//
// TODO: implement file-based storage backend
// TODO: implement cell creation, lookup, and enumeration
type Store interface {
	// Create initializes a new project cell.
	Create(config ProjectConfig) (*Cell, error)

	// Get retrieves a project cell by ID.
	Get(id string) (*Cell, error)

	// List returns all project cells in the workspace.
	List() ([]*Cell, error)
}
