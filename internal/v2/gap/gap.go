package gap

import (
	"os"
	"strings"
)

// ReadChangedFiles returns the list of files recorded in the gap analysis diff file.
func ReadChangedFiles(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var changed []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		changed = append(changed, line)
	}
	return changed, nil
}
