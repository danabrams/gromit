// Package inspect performs static analysis on a project repository to produce
// structured artifacts (architecture.json, source-map.json, glossary.json).
//
// Inspection is the primary input-gathering phase. It reads the repo and
// produces artifacts that downstream stages (guide, context) consume.
//
// TODO: implement repo inspection orchestration
// TODO: implement architecture extraction (module boundaries, dependency graph)
// TODO: implement source map generation (file inventory, language detection)
// TODO: implement glossary extraction (domain terms from code and docs)
// TODO: implement incremental inspection (diff-based updates)
package inspect

import "github.com/danabrams/gromit/internal/next/projectcell"

// Result holds the output of a full repo inspection pass.
type Result struct {
	Architecture Architecture
	SourceMap    SourceMap
	Glossary     Glossary
}

// Inspector performs repo inspection and writes artifacts to a project cell.
//
// TODO: implement inspector with configurable analysis passes
type Inspector interface {
	Inspect(cell *projectcell.Cell) (*Result, error)
}
