// internal/github/github_test.go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaz8081/proof/internal/review"
	gh "github.com/google/go-github/v68/github"
)

func testClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ghClient := gh.NewClient(nil)
	ghClient.BaseURL, _ = ghClient.BaseURL.Parse(server.URL + "/")

	return &Client{gh: ghClient}
}

func TestFindReviewRequests_FiltersByRepo(t *testing.T) {
	mux := http.NewServeMux()

	// Mock the search endpoint
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		result := &gh.IssuesSearchResult{
			Total: gh.Ptr(2),
			Issues: []*gh.Issue{
				{
					Number:           gh.Ptr(1),
					Title:            gh.Ptr("Add feature"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo-a"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo-a/pulls/1")},
					User:             &gh.User{Login: gh.Ptr("alice")},
					Draft:            gh.Ptr(false),
				},
				{
					Number:           gh.Ptr(5),
					Title:            gh.Ptr("Fix bug"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo-b"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo-b/pulls/5")},
					User:             &gh.User{Login: gh.Ptr("bob")},
					Draft:            gh.Ptr(false),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	prs, err := client.FindReviewRequests(context.Background(), []string{"owner/repo-a", "owner/repo-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Number != 1 || prs[0].Owner != "owner" || prs[0].Repo != "repo-a" {
		t.Errorf("unexpected first PR: %+v", prs[0])
	}
}

func TestFindReviewRequests_SkipsDrafts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		// Verify the query includes draft:false
		if !containsStr(q, "draft:false") {
			t.Errorf("expected query to filter drafts, got: %s", q)
		}
		result := &gh.IssuesSearchResult{Total: gh.Ptr(0), Issues: []*gh.Issue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	prs, err := client.FindReviewRequests(context.Background(), []string{"owner/repo"}, WithIgnoreDrafts(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func TestGetPRContext(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.diff" {
			w.Write([]byte("diff --git a/main.go b/main.go\n"))
			return
		}
		pr := &gh.PullRequest{
			Number: gh.Ptr(1),
			Title:  gh.Ptr("Add feature"),
			Body:   gh.Ptr("This adds a feature"),
			Head:   &gh.PullRequestBranch{Ref: gh.Ptr("feature")},
			Base:   &gh.PullRequestBranch{Ref: gh.Ptr("main")},
			User:   &gh.User{Login: gh.Ptr("alice")},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pr)
	})

	mux.HandleFunc("/repos/owner/repo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		files := []*gh.CommitFile{
			{Filename: gh.Ptr("main.go")},
			{Filename: gh.Ptr("util.go")},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	client := testClient(t, mux)
	prCtx, err := client.GetPRContext(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prCtx.Title != "Add feature" {
		t.Errorf("unexpected title: %q", prCtx.Title)
	}
	if prCtx.Diff == "" {
		t.Error("expected non-empty diff")
	}
	if len(prCtx.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(prCtx.Files))
	}
}

func TestCreatePendingReview(t *testing.T) {
	mux := http.NewServeMux()

	var capturedBody map[string]any
	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)

		rev := &gh.PullRequestReview{
			ID:    gh.Ptr(int64(42)),
			State: gh.Ptr("PENDING"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rev)
	})

	client := testClient(t, mux)
	result := &review.ReviewResult{
		Summary: "Looks good with minor issues",
		Verdict: "COMMENT",
		Comments: []review.InlineComment{
			{
				Path:     "main.go",
				Line:     10,
				Body:     "Consider error handling",
				Severity: "issue",
			},
		},
	}

	reviewID, err := client.CreatePendingReview(context.Background(), "owner", "repo", 1, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewID != 42 {
		t.Errorf("expected review ID 42, got %d", reviewID)
	}

	// Verify no event was sent (pending review)
	if event, ok := capturedBody["event"]; ok && event != "" {
		t.Errorf("expected no event for pending review, got %q", event)
	}
}

func TestSubmitReview(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews/42/events", func(w http.ResponseWriter, r *http.Request) {
		rev := &gh.PullRequestReview{
			ID:    gh.Ptr(int64(42)),
			State: gh.Ptr("APPROVED"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rev)
	})

	client := testClient(t, mux)
	err := client.SubmitReview(context.Background(), "owner", "repo", 1, 42, "APPROVE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPendingReviews(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*gh.PullRequestReview{
			{
				ID:    gh.Ptr(int64(42)),
				State: gh.Ptr("PENDING"),
				Body:  gh.Ptr("AI review summary"),
				User:  &gh.User{Login: gh.Ptr("chaz8081")},
			},
			{
				ID:    gh.Ptr(int64(43)),
				State: gh.Ptr("APPROVED"),
				Body:  gh.Ptr("LGTM"),
				User:  &gh.User{Login: gh.Ptr("alice")},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	})

	client := testClient(t, mux)
	reviews, err := client.ListPendingReviews(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("expected 1 pending review, got %d", len(reviews))
	}
	if reviews[0].ID != 42 {
		t.Errorf("expected review ID 42, got %d", reviews[0].ID)
	}
}

func TestListPendingReviews_NoPending(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*gh.PullRequestReview{
			{
				ID:    gh.Ptr(int64(43)),
				State: gh.Ptr("APPROVED"),
				Body:  gh.Ptr("LGTM"),
				User:  &gh.User{Login: gh.Ptr("alice")},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	})

	client := testClient(t, mux)
	reviews, err := client.ListPendingReviews(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("expected 0 pending reviews, got %d", len(reviews))
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
