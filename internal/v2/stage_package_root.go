package v2

import "path/filepath"

func stagePackageRoot() string {
	return filepath.Join("..", "..", "stage")
}
