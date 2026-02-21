// Package epilogue implements Stage 5 of the Gromit pipeline: bead lifecycle and cleanup.
// It closes and syncs the bead, runs the spec gate, merges interactive branches,
// writes status, and triggers thorough reviews.
package epilogue
