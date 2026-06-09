package findings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct {
	RunID string
	dir   string
	items []Finding
}

func NewStore(runID, artifactsDir string) (*Store, error) {
	dir := filepath.Join(artifactsDir, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating run dir: %w", err)
	}
	return &Store{RunID: runID, dir: dir}, nil
}

func (s *Store) Add(f Finding) {
	s.items = append(s.items, f)
}

func (s *Store) AddAll(fs []Finding) {
	s.items = append(s.items, fs...)
}

func (s *Store) All() []Finding {
	return s.items
}

func (s *Store) Flush() error {
	path := filepath.Join(s.dir, "findings.json")
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadStore(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fs []Finding
	return fs, json.Unmarshal(data, &fs)
}
