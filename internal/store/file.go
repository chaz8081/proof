package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileStore implements Store using a JSON file.
type FileStore struct {
	path string
}

// NewFileStore creates a FileStore at the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Add(record PendingRecord) error {
	records, err := s.List()
	if err != nil {
		return err
	}

	// Deduplicate: remove existing entry for same PR
	filtered := make([]PendingRecord, 0, len(records))
	for _, r := range records {
		if !(r.Owner == record.Owner && r.Repo == record.Repo && r.Number == record.Number) {
			filtered = append(filtered, r)
		}
	}
	filtered = append(filtered, record)

	return s.write(filtered)
}

func (s *FileStore) List() ([]PendingRecord, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []PendingRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *FileStore) Remove(owner string, repo string, number int) error {
	records, err := s.List()
	if err != nil {
		return err
	}

	filtered := make([]PendingRecord, 0, len(records))
	for _, r := range records {
		if !(r.Owner == owner && r.Repo == repo && r.Number == number) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		// Remove the file if empty
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	return s.write(filtered)
}

func (s *FileStore) write(records []PendingRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
