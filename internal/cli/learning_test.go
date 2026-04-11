package cli

import (
	"testing"

	proofstore "github.com/chaz8081/proof/internal/store"
)

func TestComputeDelta_NoChanges(t *testing.T) {
	original := &proofstore.OriginalReview{
		Verdict:      "COMMENT",
		CommentCount: 3,
	}
	delta := computeDelta(original, 3, "COMMENT")

	if delta.OriginalComments != 3 {
		t.Errorf("OriginalComments: got %d, want 3", delta.OriginalComments)
	}
	if delta.SubmittedComments != 3 {
		t.Errorf("SubmittedComments: got %d, want 3", delta.SubmittedComments)
	}
	if delta.Deleted != 0 {
		t.Errorf("Deleted: got %d, want 0", delta.Deleted)
	}
	if delta.VerdictChanged {
		t.Error("VerdictChanged: got true, want false")
	}
	if delta.OriginalVerdict != "COMMENT" {
		t.Errorf("OriginalVerdict: got %q, want %q", delta.OriginalVerdict, "COMMENT")
	}
	if delta.SubmittedVerdict != "COMMENT" {
		t.Errorf("SubmittedVerdict: got %q, want %q", delta.SubmittedVerdict, "COMMENT")
	}
}

func TestComputeDelta_CommentsDeleted(t *testing.T) {
	original := &proofstore.OriginalReview{
		Verdict:      "REQUEST_CHANGES",
		CommentCount: 5,
	}
	delta := computeDelta(original, 3, "REQUEST_CHANGES")

	if delta.Deleted != 2 {
		t.Errorf("Deleted: got %d, want 2", delta.Deleted)
	}
	if delta.VerdictChanged {
		t.Error("VerdictChanged: got true, want false")
	}
}

func TestComputeDelta_VerdictChanged(t *testing.T) {
	original := &proofstore.OriginalReview{
		Verdict:      "REQUEST_CHANGES",
		CommentCount: 2,
	}
	delta := computeDelta(original, 2, "APPROVE")

	if !delta.VerdictChanged {
		t.Error("VerdictChanged: got false, want true")
	}
	if delta.OriginalVerdict != "REQUEST_CHANGES" {
		t.Errorf("OriginalVerdict: got %q, want %q", delta.OriginalVerdict, "REQUEST_CHANGES")
	}
	if delta.SubmittedVerdict != "APPROVE" {
		t.Errorf("SubmittedVerdict: got %q, want %q", delta.SubmittedVerdict, "APPROVE")
	}
}

func TestComputeDelta_UserAddedComments(t *testing.T) {
	// submitted > original: Deleted should clamp to 0
	original := &proofstore.OriginalReview{
		Verdict:      "COMMENT",
		CommentCount: 2,
	}
	delta := computeDelta(original, 5, "COMMENT")

	if delta.Deleted != 0 {
		t.Errorf("Deleted: got %d, want 0 (user added comments, no negative deletion)", delta.Deleted)
	}
	if delta.SubmittedComments != 5 {
		t.Errorf("SubmittedComments: got %d, want 5", delta.SubmittedComments)
	}
}

func TestComputeDelta_AllCommentsDeleted(t *testing.T) {
	original := &proofstore.OriginalReview{
		Verdict:      "COMMENT",
		CommentCount: 4,
	}
	delta := computeDelta(original, 0, "APPROVE")

	if delta.Deleted != 4 {
		t.Errorf("Deleted: got %d, want 4", delta.Deleted)
	}
	if !delta.VerdictChanged {
		t.Error("VerdictChanged: got false, want true")
	}
}

func TestComputeDelta_TimestampSet(t *testing.T) {
	original := &proofstore.OriginalReview{Verdict: "COMMENT", CommentCount: 1}
	delta := computeDelta(original, 1, "COMMENT")
	if delta.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}
