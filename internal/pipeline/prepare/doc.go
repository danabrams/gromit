// Package prepare implements Stage 1 of the Gromit pipeline: gate decisions.
// It runs precheck, stuck-bead detection, and scope gate.
// Returns a Decision (Proceed, Skip, or Block) before any LLM invocation.
package prepare
