package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileStore(filepath.Join(dir, "pending.json"))
}

func TestFileStore_AddAndList(t *testing.T) {
	s := newTestStore(t)

	r1 := PendingRecord{Owner: "acme", Repo: "alpha", Number: 1, ReviewID: 100, Created: time.Now()}
	r2 := PendingRecord{Owner: "acme", Repo: "beta", Number: 2, ReviewID: 200, Created: time.Now()}

	if err := s.Add(r1); err != nil {
		t.Fatalf("Add r1: %v", err)
	}
	if err := s.Add(r2); err != nil {
		t.Fatalf("Add r2: %v", err)
	}

	records, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestFileStore_AddDeduplicates(t *testing.T) {
	s := newTestStore(t)

	r1 := PendingRecord{Owner: "acme", Repo: "alpha", Number: 1, ReviewID: 100, Created: time.Now()}
	r2 := PendingRecord{Owner: "acme", Repo: "alpha", Number: 1, ReviewID: 999, Created: time.Now()}

	if err := s.Add(r1); err != nil {
		t.Fatalf("Add r1: %v", err)
	}
	if err := s.Add(r2); err != nil {
		t.Fatalf("Add r2: %v", err)
	}

	records, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after deduplication, got %d", len(records))
	}
	if records[0].ReviewID != 999 {
		t.Errorf("expected ReviewID 999, got %d", records[0].ReviewID)
	}
}

func TestFileStore_RemoveExisting(t *testing.T) {
	s := newTestStore(t)

	r1 := PendingRecord{Owner: "acme", Repo: "alpha", Number: 1, ReviewID: 100, Created: time.Now()}
	r2 := PendingRecord{Owner: "acme", Repo: "beta", Number: 2, ReviewID: 200, Created: time.Now()}

	if err := s.Add(r1); err != nil {
		t.Fatalf("Add r1: %v", err)
	}
	if err := s.Add(r2); err != nil {
		t.Fatalf("Add r2: %v", err)
	}

	if err := s.Remove("acme", "alpha", 1); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	records, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Repo != "beta" {
		t.Errorf("expected remaining record to be beta, got %s", records[0].Repo)
	}
}

func TestFileStore_RemoveNonExistent(t *testing.T) {
	s := newTestStore(t)

	r1 := PendingRecord{Owner: "acme", Repo: "alpha", Number: 1, ReviewID: 100, Created: time.Now()}
	if err := s.Add(r1); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Removing a non-existent entry should not return an error
	if err := s.Remove("acme", "nonexistent", 99); err != nil {
		t.Fatalf("Remove non-existent: %v", err)
	}

	records, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record to remain, got %d", len(records))
	}
}

func TestFileStore_ListEmptyWhenNoFile(t *testing.T) {
	s := newTestStore(t)

	records, err := s.List()
	if err != nil {
		t.Fatalf("List on missing file: %v", err)
	}
	if records != nil {
		t.Fatalf("expected nil records, got %v", records)
	}
}

func TestFileStore_RemoveLastCleansUpFile(t *testing.T) {
	s := newTestStore(t)

	r1 := PendingRecord{Owner: "acme", Repo: "alpha", Number: 1, ReviewID: 100, Created: time.Now()}
	if err := s.Add(r1); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := s.Remove("acme", "alpha", 1); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// File should no longer exist
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted after removing last entry, but it still exists")
	}
}

func TestFileStore_HeadSHAStoredAndRetrieved(t *testing.T) {
	s := newTestStore(t)

	r := PendingRecord{
		Owner:    "acme",
		Repo:     "alpha",
		Number:   1,
		ReviewID: 100,
		Created:  time.Now(),
		HeadSHA:  "abc1234def5678",
		OriginalResult: &OriginalReview{
			Summary:      "Looks good overall",
			Verdict:      "APPROVE",
			CommentCount: 2,
			CommentPaths: []string{"main.go", "util.go"},
		},
	}

	if err := s.Add(r); err != nil {
		t.Fatalf("Add: %v", err)
	}

	records, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	got := records[0]
	if got.HeadSHA != "abc1234def5678" {
		t.Errorf("expected HeadSHA %q, got %q", "abc1234def5678", got.HeadSHA)
	}
	if got.OriginalResult == nil {
		t.Fatal("expected OriginalResult to be non-nil")
	}
	if got.OriginalResult.Summary != "Looks good overall" {
		t.Errorf("expected Summary %q, got %q", "Looks good overall", got.OriginalResult.Summary)
	}
	if got.OriginalResult.Verdict != "APPROVE" {
		t.Errorf("expected Verdict %q, got %q", "APPROVE", got.OriginalResult.Verdict)
	}
	if got.OriginalResult.CommentCount != 2 {
		t.Errorf("expected CommentCount 2, got %d", got.OriginalResult.CommentCount)
	}
	if len(got.OriginalResult.CommentPaths) != 2 {
		t.Errorf("expected 2 comment paths, got %d", len(got.OriginalResult.CommentPaths))
	}
}

func TestFileStore_HeadSHA_UpdatedOnReReview(t *testing.T) {
	s := newTestStore(t)

	// First review
	r1 := PendingRecord{
		Owner:    "acme",
		Repo:     "alpha",
		Number:   1,
		ReviewID: 100,
		Created:  time.Now(),
		HeadSHA:  "sha-v1",
	}
	if err := s.Add(r1); err != nil {
		t.Fatalf("Add r1: %v", err)
	}

	// Re-review updates the record including the new HeadSHA
	r2 := PendingRecord{
		Owner:    "acme",
		Repo:     "alpha",
		Number:   1,
		ReviewID: 200,
		Created:  time.Now(),
		HeadSHA:  "sha-v2",
	}
	if err := s.Add(r2); err != nil {
		t.Fatalf("Add r2: %v", err)
	}

	records, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after deduplication, got %d", len(records))
	}
	if records[0].HeadSHA != "sha-v2" {
		t.Errorf("expected updated HeadSHA %q, got %q", "sha-v2", records[0].HeadSHA)
	}
	if records[0].ReviewID != 200 {
		t.Errorf("expected updated ReviewID 200, got %d", records[0].ReviewID)
	}
}
