// Package pipeline defines the stage interfaces for the Gromit runner pipeline.
// Each stage receives an Input, performs its work, and returns an Output.
// The Runner orchestrator in internal/runner/ wires stages together.
package pipeline
