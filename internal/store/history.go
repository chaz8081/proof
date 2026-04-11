package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ReviewRecord captures the outcome of a single AI review run for history and analytics.
type ReviewRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	Owner        string    `json:"owner"`
	Repo         string    `json:"repo"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Verdict      string    `json:"verdict"`
	CommentCount int       `json:"comment_count"`
	FileCount    int       `json:"file_count"`
	DiffBytes    int       `json:"diff_bytes"`
	Model        string    `json:"model"`
	ReviewID     int64     `json:"review_id"`
	Duration     float64   `json:"duration_seconds"`
}

// HistoryStore manages the append-only review history log.
type HistoryStore struct {
	path string
}

// NewHistoryStore creates a HistoryStore backed by the given file path.
func NewHistoryStore(path string) *HistoryStore {
	return &HistoryStore{path: path}
}

// Append writes a single ReviewRecord as a JSONL line to the history file.
func (h *HistoryStore) Append(record ReviewRecord) error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(record)
}

// List returns all ReviewRecords from the history file.
// Returns nil (not an error) when the file does not yet exist.
func (h *HistoryStore) List() ([]ReviewRecord, error) {
	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []ReviewRecord
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r ReviewRecord
		if err := json.Unmarshal(line, &r); err != nil {
			continue // skip malformed lines
		}
		records = append(records, r)
	}
	return records, nil
}

// ListForPR returns all ReviewRecords for a specific PR.
func (h *HistoryStore) ListForPR(owner, repo string, number int) ([]ReviewRecord, error) {
	all, err := h.List()
	if err != nil {
		return nil, err
	}
	var filtered []ReviewRecord
	for _, r := range all {
		if r.Owner == owner && r.Repo == repo && r.Number == number {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}
