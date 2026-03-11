package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// Bundler assembles evidence files into a directory that documents
// what happened during a spec execution run.
type Bundler struct {
	dir string
}

// NewBundler creates a Bundler that writes evidence files to dir.
func NewBundler(dir string) *Bundler {
	return &Bundler{dir: dir}
}

// Init creates the evidence directory.
func (b *Bundler) Init() error {
	return os.MkdirAll(b.dir, 0o755)
}

// WriteTaskResults writes task results to task-results.json.
func (b *Bundler) WriteTaskResults(tasks []runstore.Task) error {
	return b.writeJSON("task-results.json", tasks)
}

func (b *Bundler) writeJSON(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.dir, name), data, 0o644)
}
