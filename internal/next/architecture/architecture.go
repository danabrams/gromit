// Package architecture provides higher-level architectural reasoning
// on top of the raw inspection artifacts.
//
// This package consumes architecture.json and source-map.json to answer
// questions like "which modules are affected by this change?" or
// "what is the dependency fan-out of this package?"
//
// TODO: implement dependency graph traversal
// TODO: implement change-impact analysis
// TODO: implement module boundary validation
package architecture

// Module represents a logical module boundary in the codebase.
type Module struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
}

// Dependency represents a directional dependency between modules.
type Dependency struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Component groups related modules into a higher-level component.
type Component struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Modules     []string `json:"modules"`
}

// Architecture holds the full architectural model of a codebase.
type Architecture struct {
	Modules      []Module     `json:"modules"`
	Dependencies []Dependency `json:"dependencies"`
	Components   []Component  `json:"components"`
}

// New returns an Architecture with initialized (non-nil) slices.
func New() Architecture {
	return Architecture{
		Modules:      []Module{},
		Dependencies: []Dependency{},
		Components:   []Component{},
	}
}

// AddModule appends a module to the architecture.
func (a *Architecture) AddModule(m Module) {
	a.Modules = append(a.Modules, m)
}

// AddDependency appends a dependency to the architecture.
func (a *Architecture) AddDependency(d Dependency) {
	a.Dependencies = append(a.Dependencies, d)
}

// AddComponent appends a component to the architecture.
func (a *Architecture) AddComponent(c Component) {
	a.Components = append(a.Components, c)
}

// NormalizeNilFields maps nil slices to empty values.
func (a *Architecture) NormalizeNilFields() {
	if a.Modules == nil {
		a.Modules = []Module{}
	}
	if a.Dependencies == nil {
		a.Dependencies = []Dependency{}
	}
	if a.Components == nil {
		a.Components = []Component{}
	}
	for i := range a.Components {
		if a.Components[i].Modules == nil {
			a.Components[i].Modules = []string{}
		}
	}
}
