// Package sourcemap provides utilities for working with the source-map.json
// artifact produced by inspection.
//
// Source maps are used to select relevant files for context compilation,
// estimate token budgets, and detect structural changes between inspections.
//
// TODO: implement source map diffing (detect added/removed/renamed files)
// TODO: implement file selection by language, path glob, or role
// TODO: implement token budget estimation from file sizes
package sourcemap
