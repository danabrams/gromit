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
