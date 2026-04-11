package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryStore_AppendAndList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviews.jsonl")
	h := NewHistoryStore(path)

	now := time.Now().Truncate(time.Second)
	records := []ReviewRecord{
		{
			Timestamp:    now,
			Owner:        "acme",
			Repo:         "api",
			Number:       1,
			Title:        "First PR",
			Author:       "alice",
			Verdict:      "COMMENT",
			CommentCount: 3,
			FileCount:    2,
			DiffBytes:    512,
			Model:        "gpt-4.1",
			ReviewID:     101,
			Duration:     4.5,
		},
		{
			Timestamp:    now,
			Owner:        "acme",
			Repo:         "api",
			Number:       2,
			Title:        "Second PR",
			Author:       "bob",
			Verdict:      "APPROVE",
			CommentCount: 0,
			FileCount:    1,
			DiffBytes:    128,
			Model:        "gpt-4.1",
			ReviewID:     102,
			Duration:     2.1,
		},
	}

	for _, r := range records {
		if err := h.Append(r); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	got, err := h.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	if got[0].Number != 1 || got[1].Number != 2 {
		t.Errorf("unexpected record order: %v, %v", got[0].Number, got[1].Number)
	}
	if got[0].Verdict != "COMMENT" {
		t.Errorf("expected COMMENT, got %q", got[0].Verdict)
	}
	if got[1].Author != "bob" {
		t.Errorf("expected bob, got %q", got[1].Author)
	}
}

func TestHistoryStore_ListForPR(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviews.jsonl")
	h := NewHistoryStore(path)

	now := time.Now()
	for _, r := range []ReviewRecord{
		{Owner: "org", Repo: "alpha", Number: 10, Verdict: "APPROVE", Timestamp: now, Model: "m"},
		{Owner: "org", Repo: "alpha", Number: 10, Verdict: "COMMENT", Timestamp: now, Model: "m"},
		{Owner: "org", Repo: "beta", Number: 10, Verdict: "REQUEST_CHANGES", Timestamp: now, Model: "m"},
		{Owner: "org", Repo: "alpha", Number: 11, Verdict: "APPROVE", Timestamp: now, Model: "m"},
	} {
		if err := h.Append(r); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	got, err := h.ListForPR("org", "alpha", 10)
	if err != nil {
		t.Fatalf("ListForPR failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records for org/alpha#10, got %d", len(got))
	}
	for _, r := range got {
		if r.Owner != "org" || r.Repo != "alpha" || r.Number != 10 {
			t.Errorf("unexpected record in result: %+v", r)
		}
	}
}

func TestHistoryStore_EmptyWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	h := NewHistoryStore(filepath.Join(dir, "nonexistent.jsonl"))

	got, err := h.List()
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil slice for missing file, got %v", got)
	}
}

func TestHistoryStore_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviews.jsonl")

	// Write one valid and one malformed JSONL line
	content := `{"timestamp":"2026-04-09T00:00:00Z","owner":"o","repo":"r","number":1,"verdict":"APPROVE","model":"m"}
NOT VALID JSON
{"timestamp":"2026-04-09T00:00:00Z","owner":"o","repo":"r","number":2,"verdict":"COMMENT","model":"m"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	h := NewHistoryStore(path)
	got, err := h.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 valid records (malformed skipped), got %d", len(got))
	}
	if got[0].Number != 1 || got[1].Number != 2 {
		t.Errorf("unexpected records: %v, %v", got[0].Number, got[1].Number)
	}
}
