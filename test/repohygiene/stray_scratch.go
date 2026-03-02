package repohygiene

import (
	"os"
	"path/filepath"
)

func findStrayScratchFiles(repoRoot string, candidates []string) ([]string, error) {
	var found []string
	for _, name := range candidates {
		path := filepath.Join(repoRoot, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, name)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return found, nil
}
