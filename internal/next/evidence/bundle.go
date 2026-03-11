package evidence

import "os"

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
