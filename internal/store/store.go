package store

import "time"

// OriginalReview captures the AI-generated review at creation time for learning comparisons.
type OriginalReview struct {
	Summary      string   `json:"summary"`
	Verdict      string   `json:"verdict"`
	CommentCount int      `json:"comment_count"`
	CommentPaths []string `json:"comment_paths"` // just the file paths for comparison
}

// PendingRecord tracks a pending review that proof created on GitHub.
type PendingRecord struct {
	Owner          string          `json:"owner"`
	Repo           string          `json:"repo"`
	Number         int             `json:"number"`
	ReviewID       int64           `json:"review_id"`
	Created        time.Time       `json:"created"`
	HeadSHA        string          `json:"head_sha,omitempty"`
	OriginalResult *OriginalReview `json:"original_result,omitempty"`
}

// Store manages the local record of pending reviews created by proof.
type Store interface {
	// Add records a newly-created pending review.
	Add(record PendingRecord) error

	// List returns all tracked pending reviews.
	List() ([]PendingRecord, error)

	// Remove deletes the record for the given PR.
	// Returns nil if the record does not exist (idempotent).
	Remove(owner string, repo string, number int) error
}
