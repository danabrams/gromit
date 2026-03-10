package inspect

// Architecture represents the architecture.json artifact.
//
// Captures module boundaries, dependency relationships, and key abstractions
// discovered during repo inspection.
//
// TODO: define full schema based on inspection requirements
type Architecture struct {
	// Modules is the list of discovered architectural modules.
	Modules []Module `json:"modules"`
}

// Module represents a single architectural module or package boundary.
type Module struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// SourceMap represents the source-map.json artifact.
//
// Provides a structured inventory of all source files, their languages,
// and their roles within the project.
//
// TODO: define full schema (file roles, size tiers, change frequency)
type SourceMap struct {
	// Files is the list of source files in the repository.
	Files []SourceFile `json:"files"`
}

// SourceFile represents a single file entry in the source map.
type SourceFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
}

// Glossary represents the glossary.json artifact.
//
// Captures domain-specific terminology found in code and documentation.
//
// TODO: define full schema (term sources, confidence scores)
type Glossary struct {
	// Terms is the list of domain terms.
	Terms []GlossaryTerm `json:"terms"`
}

// GlossaryTerm is a single domain term with its definition.
type GlossaryTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}
