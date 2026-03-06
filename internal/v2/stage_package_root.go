package v2

import (
	"path/filepath"
	"runtime"
)

func stagePackageRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("..", "..", "stage")
	}
	stageDir := filepath.Join(filepath.Dir(file), "stage")
	if absDir, err := filepath.Abs(stageDir); err == nil {
		return absDir
	}
	return stageDir
}
