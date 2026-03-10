// Package guide renders agent-guide.md from inspection artifacts and doctrine.
//
// The agent guide is a compiled markdown document that provides an LLM agent
// with everything it needs to work effectively in a project: architecture
// overview, key conventions, file inventory, and domain glossary.
//
// TODO: implement guide rendering from artifacts
// TODO: implement guide sectioning (architecture, conventions, glossary, file map)
// TODO: implement guide token budget management
// TODO: implement incremental guide updates
package guide

// Input holds everything needed to render an agent guide.
//
// TODO: populate from inspection result + doctrine
type Input struct {
	// ProjectName is the human-readable project name.
	ProjectName string

	// Architecture is the raw architecture description.
	Architecture string

	// Conventions is the rendered doctrine/rules section.
	Conventions string

	// FileMap is the rendered source map section.
	FileMap string

	// Glossary is the rendered glossary section.
	Glossary string
}

// Renderer produces agent-guide.md from an Input.
//
// TODO: implement markdown rendering with section headers
// TODO: implement token budget enforcement
type Renderer interface {
	Render(input Input) (string, error)
}
