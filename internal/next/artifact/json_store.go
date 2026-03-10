package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type JSONStore struct{}

func NewJSONStore() *JSONStore {
	return &JSONStore{}
}

func (s *JSONStore) Read(cellPath string, artifact string, dest any) error {
	data, err := os.ReadFile(s.path(cellPath, artifact))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (s *JSONStore) Write(cellPath string, artifact string, src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	p := s.path(cellPath, artifact)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (s *JSONStore) Exists(cellPath string, artifact string) bool {
	_, err := os.Stat(s.path(cellPath, artifact))
	return err == nil
}

func (s *JSONStore) path(cellPath string, artifact string) string {
	return filepath.Join(cellPath, artifact+".json")
}
