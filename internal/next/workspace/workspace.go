// Package workspace resolves and manages the Gromit workspace root.
//
// The workspace is the top-level directory that contains all project cells.
// Unlike legacy Gromit, the workspace is NOT a repo-local .gromit/ directory —
// it is a user-scoped location (e.g. ~/.gromit/ or XDG-based).
//
// TODO: implement workspace root resolution (XDG_DATA_HOME, fallback to ~/.gromit)
// TODO: implement workspace initialization (create directory structure on first use)
// TODO: implement workspace locking for concurrent CLI invocations
package workspace

// Root represents a resolved workspace root directory.
type Root struct {
	// Path is the absolute path to the workspace root.
	Path string
}

// Resolver finds or initializes the workspace root.
//
// TODO: implement resolution strategy:
//   - check GROMIT_WORKSPACE env var
//   - check XDG_DATA_HOME/gromit
//   - fall back to ~/.gromit-next (avoiding collision with legacy ~/.gromit)
type Resolver interface {
	Resolve() (Root, error)
}
