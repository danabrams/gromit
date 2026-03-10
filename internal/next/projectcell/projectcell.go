package projectcell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Cell struct {
	Name      string    `json:"name"`
	RepoPath  string    `json:"repo_path"`
	CreatedAt time.Time `json:"created_at"`
	CellPath  string    `json:"-"`
}

type Store interface {
	Create(name string, repoPath string) (Cell, error)
	Get(name string) (Cell, error)
	List() ([]Cell, error)
	Delete(name string) error
}

type FSStore struct {
	projectsDir string
}

func NewFSStore(projectsDir string) *FSStore {
	return &FSStore{projectsDir: projectsDir}
}

func (s *FSStore) Create(name string, repoPath string) (Cell, error) {
	cellDir := filepath.Join(s.projectsDir, name)
	if _, err := os.Stat(cellDir); err == nil {
		return Cell{}, fmt.Errorf("project %q already exists", name)
	}
	if !isGitRepo(repoPath) {
		return Cell{}, fmt.Errorf("%q is not a git repository", repoPath)
	}

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return Cell{}, err
	}

	cell := Cell{
		Name:      name,
		RepoPath:  abs,
		CreatedAt: time.Now(),
		CellPath:  cellDir,
	}

	for _, sub := range []string{"artifacts", "doctrine", "provenance", "guide"} {
		if err := os.MkdirAll(filepath.Join(cellDir, sub), 0o755); err != nil {
			return Cell{}, err
		}
	}

	data, err := json.MarshalIndent(cell, "", "  ")
	if err != nil {
		return Cell{}, err
	}
	return cell, os.WriteFile(filepath.Join(cellDir, "project.json"), data, 0o644)
}

func (s *FSStore) Get(name string) (Cell, error) {
	cellDir := filepath.Join(s.projectsDir, name)
	data, err := os.ReadFile(filepath.Join(cellDir, "project.json"))
	if err != nil {
		return Cell{}, fmt.Errorf("project %q not found: %w", name, err)
	}
	var cell Cell
	if err := json.Unmarshal(data, &cell); err != nil {
		return Cell{}, err
	}
	cell.CellPath = cellDir
	return cell, nil
}

func (s *FSStore) List() ([]Cell, error) {
	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cells []Cell
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cell, err := s.Get(e.Name())
		if err != nil {
			continue
		}
		cells = append(cells, cell)
	}
	return cells, nil
}

func (s *FSStore) Delete(name string) error {
	cellDir := filepath.Join(s.projectsDir, name)
	return os.RemoveAll(cellDir)
}

func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
