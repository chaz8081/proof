// internal/github/github_test.go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
