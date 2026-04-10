// internal/github/integration_test.go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chaz8081/proof/internal/review"
	gh "github.com/google/go-github/v68/github"
)

func TestFullReviewWorkflow(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Mock search endpoint
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		result := &gh.IssuesSearchResult{
			Total: gh.Ptr(1),
			Issues: []*gh.Issue{
				{
					Number:        gh.Ptr(42),
					Title:         gh.Ptr("Add feature X"),
					RepositoryURL: gh.Ptr("https://api.github.com/repos/test/repo"),
					PullRequestLinks: &gh.PullRequestLinks{
						URL: gh.Ptr("https://api.github.com/repos/test/repo/pulls/42"),
					},
					User:  &gh.User{Login: gh.Ptr("author")},
					Draft: gh.Ptr(false),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// 2. Mock PR details + diff
	mux.HandleFunc("/repos/test/repo/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.diff" {
			w.Write([]byte("diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,5 @@\n package main\n+\n+func hello() {}\n"))
			return
		}
		pr := &gh.PullRequest{
			Number: gh.Ptr(42),
			Title:  gh.Ptr("Add feature X"),
			Body:   gh.Ptr("Adds hello function"),
			User:   &gh.User{Login: gh.Ptr("author")},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pr)
	})

	// 3. Mock file list
	mux.HandleFunc("/repos/test/repo/pulls/42/files", func(w http.ResponseWriter, r *http.Request) {
		files := []*gh.CommitFile{{Filename: gh.Ptr("main.go")}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	// 4. Mock list reviews (GET) and create review (POST)
	mux.HandleFunc("/repos/test/repo/pulls/42/reviews", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			rev := &gh.PullRequestReview{
				ID:    gh.Ptr(int64(999)),
				State: gh.Ptr("PENDING"),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rev)
			return
		}
		// GET — no existing reviews
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.PullRequestReview{})
	})

	client := testClient(t, mux)

	// Step 1: Find PRs
	prs, err := client.FindReviewRequests(context.Background(), []string{"test/repo"})
	if err != nil {
		t.Fatalf("FindReviewRequests: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 42 {
		t.Fatalf("expected PR #42, got #%d", prs[0].Number)
	}
	if prs[0].Owner != "test" || prs[0].Repo != "repo" {
		t.Errorf("unexpected owner/repo: %s/%s", prs[0].Owner, prs[0].Repo)
	}

	// Step 2: Get context
	prCtx, err := client.GetPRContext(context.Background(), "test", "repo", 42)
	if err != nil {
		t.Fatalf("GetPRContext: %v", err)
	}
	if prCtx.Title != "Add feature X" {
		t.Errorf("unexpected title: %q", prCtx.Title)
	}
	if prCtx.Diff == "" {
		t.Error("expected non-empty diff")
	}
	if len(prCtx.Files) != 1 || prCtx.Files[0] != "main.go" {
		t.Errorf("unexpected files: %v", prCtx.Files)
	}

	// Step 3: Verify no existing pending reviews
	pending, err := client.ListPendingReviews(context.Background(), "test", "repo", 42)
	if err != nil {
		t.Fatalf("ListPendingReviews: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending reviews, got %d", len(pending))
	}

	// Step 4: Mock review result (simulating what the AI would return)
	result := &review.ReviewResult{
		Summary: "Clean addition of hello function",
		Verdict: "APPROVE",
		Comments: []review.InlineComment{
			{Path: "main.go", Line: 4, Body: "Consider adding a doc comment", Severity: "nit"},
		},
	}

	// Step 5: Create pending review
	reviewID, err := client.CreatePendingReview(context.Background(), "test", "repo", 42, result, prCtx.Diff)
	if err != nil {
		t.Fatalf("CreatePendingReview: %v", err)
	}
	if reviewID != 999 {
		t.Errorf("expected review ID 999, got %d", reviewID)
	}
}
