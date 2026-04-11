package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/chaz8081/proof/internal/config"
	"github.com/chaz8081/proof/internal/review"
	proofstore "github.com/chaz8081/proof/internal/store"
)

// extractPaths returns the unique file paths from a slice of inline comments.
func extractPaths(comments []review.InlineComment) []string {
	seen := make(map[string]struct{}, len(comments))
	paths := make([]string, 0, len(comments))
	for _, c := range comments {
		if _, ok := seen[c.Path]; !ok {
			seen[c.Path] = struct{}{}
			paths = append(paths, c.Path)
		}
	}
	return paths
}

// LearningDelta captures the difference between the AI-generated review and the
// version actually submitted by the user, for quality tracking over time.
type LearningDelta struct {
	Timestamp         time.Time `json:"timestamp"`
	Owner             string    `json:"owner"`
	Repo              string    `json:"repo"`
	Number            int       `json:"number"`
	OriginalComments  int       `json:"original_comments"`
	SubmittedComments int       `json:"submitted_comments"`
	Deleted           int       `json:"deleted"`
	VerdictChanged    bool      `json:"verdict_changed"`
	OriginalVerdict   string    `json:"original_verdict"`
	SubmittedVerdict  string    `json:"submitted_verdict"`
}

// computeDelta builds a LearningDelta comparing the original AI review to what was submitted.
func computeDelta(original *proofstore.OriginalReview, submittedCount int, submittedVerdict string) LearningDelta {
	deleted := original.CommentCount - submittedCount
	if deleted < 0 {
		deleted = 0 // user added comments
	}
	return LearningDelta{
		Timestamp:         time.Now(),
		OriginalComments:  original.CommentCount,
		SubmittedComments: submittedCount,
		Deleted:           deleted,
		VerdictChanged:    original.Verdict != submittedVerdict,
		OriginalVerdict:   original.Verdict,
		SubmittedVerdict:  submittedVerdict,
	}
}

// saveLearningDelta appends a delta record to ~/.proof/learning.jsonl.
func saveLearningDelta(delta LearningDelta) error {
	path := filepath.Join(config.ConfigDir(), "learning.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(delta)
}
