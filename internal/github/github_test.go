// internal/github/github_test.go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		if !strings.Contains(q, "draft:false") {
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

	reviewID, err := client.CreatePendingReview(context.Background(), "owner", "repo", 1, result, "")
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

func TestFindReviewRequests_WithTeams(t *testing.T) {
	mux := http.NewServeMux()

	// Track which queries were received.
	var receivedQueries []string

	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		receivedQueries = append(receivedQueries, q)

		var issues []*gh.Issue

		switch {
		case strings.Contains(q, "review-requested:@me"):
			// Personal review request — returns PR #1
			issues = []*gh.Issue{
				{
					Number:           gh.Ptr(1),
					Title:            gh.Ptr("Personal PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/1")},
					User:             &gh.User{Login: gh.Ptr("alice")},
					Draft:            gh.Ptr(false),
				},
				// PR #2 also shows up in the personal query — will be deduped against team result
				{
					Number:           gh.Ptr(2),
					Title:            gh.Ptr("Shared PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/2")},
					User:             &gh.User{Login: gh.Ptr("bob")},
					Draft:            gh.Ptr(false),
				},
			}
		case strings.Contains(q, "team-review-requested:myorg/myteam"):
			// Team query — returns PR #2 (duplicate) and PR #3 (new)
			issues = []*gh.Issue{
				{
					Number:           gh.Ptr(2),
					Title:            gh.Ptr("Shared PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/2")},
					User:             &gh.User{Login: gh.Ptr("bob")},
					Draft:            gh.Ptr(false),
				},
				{
					Number:           gh.Ptr(3),
					Title:            gh.Ptr("Team-only PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/3")},
					User:             &gh.User{Login: gh.Ptr("carol")},
					Draft:            gh.Ptr(false),
				},
			}
		}

		result := &gh.IssuesSearchResult{
			Total:  gh.Ptr(len(issues)),
			Issues: issues,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	prs, err := client.FindReviewRequests(
		context.Background(),
		[]string{"owner/repo"},
		WithTeams([]string{"myorg/myteam"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have exactly 3 unique PRs (not 4 — PR #2 deduplicated).
	if len(prs) != 3 {
		t.Fatalf("expected 3 PRs after deduplication, got %d: %+v", len(prs), prs)
	}

	// Verify both queries were sent.
	if len(receivedQueries) != 2 {
		t.Fatalf("expected 2 search queries, got %d: %v", len(receivedQueries), receivedQueries)
	}

	hasPersonalQuery := false
	hasTeamQuery := false
	for _, q := range receivedQueries {
		if strings.Contains(q, "review-requested:@me") {
			hasPersonalQuery = true
		}
		if strings.Contains(q, "team-review-requested:myorg/myteam") {
			hasTeamQuery = true
		}
	}
	if !hasPersonalQuery {
		t.Errorf("expected a review-requested:@me query, got: %v", receivedQueries)
	}
	if !hasTeamQuery {
		t.Errorf("expected a team-review-requested:myorg/myteam query, got: %v", receivedQueries)
	}

	// Verify PR numbers: 1, 2, 3 (order: personal first, then unique team additions).
	prNums := make(map[int]bool)
	for _, pr := range prs {
		prNums[pr.Number] = true
	}
	for _, expected := range []int{1, 2, 3} {
		if !prNums[expected] {
			t.Errorf("expected PR #%d in results", expected)
		}
	}
}

func TestFindReviewRequests_WithTeams_DraftFilter(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "draft:false") {
			t.Errorf("expected draft:false in query %q", q)
		}
		result := &gh.IssuesSearchResult{Total: gh.Ptr(0), Issues: []*gh.Issue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	_, err := client.FindReviewRequests(
		context.Background(),
		[]string{"owner/repo"},
		WithIgnoreDrafts(true),
		WithTeams([]string{"myorg/myteam"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeletePendingReview(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		review := &gh.PullRequestReview{ID: gh.Ptr(int64(42)), State: gh.Ptr("PENDING")}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(review)
	})
	client := testClient(t, mux)
	err := client.DeletePendingReview(context.Background(), "owner", "repo", 1, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindReviewRequests_WithTeams_Empty(t *testing.T) {
	mux := http.NewServeMux()

	callCount := 0
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		result := &gh.IssuesSearchResult{Total: gh.Ptr(0), Issues: []*gh.Issue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	_, err := client.FindReviewRequests(
		context.Background(),
		[]string{"owner/repo"},
		WithTeams([]string{}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No teams configured — only one query (personal) should be sent.
	if callCount != 1 {
		t.Errorf("expected 1 search query with no teams, got %d", callCount)
	}
}

func TestParseDiffLines(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,4 +1,5 @@
 package main

+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello")
 }
diff --git a/util.go b/util.go
--- a/util.go
+++ b/util.go
@@ -10,3 +10,4 @@
 func helper() {
 	return
+	// new comment
 }
`

	validLines := parseDiffLines(diff)

	// main.go hunk @@ -1,4 +1,5 @@ starting at new-file line 1:
	//   line 1: context "package main"
	//   line 2: added "import \"fmt\""
	//   line 3: added blank
	//   line 4: context "func main() {"
	//   line 5: added "fmt.Println(...)"
	//   line 6: context "}"
	mainLines, ok := validLines["main.go"]
	if !ok {
		t.Fatal("expected main.go in valid lines")
	}
	for _, ln := range []int{1, 2, 3, 4, 5, 6} {
		if !mainLines[ln] {
			t.Errorf("expected main.go line %d to be valid", ln)
		}
	}
	if mainLines[7] {
		t.Error("line 7 should not be valid in main.go (outside hunk)")
	}

	// util.go: lines 10, 11, 12, 13 should be present
	utilLines, ok := validLines["util.go"]
	if !ok {
		t.Fatal("expected util.go in valid lines")
	}
	for _, ln := range []int{10, 11, 12, 13} {
		if !utilLines[ln] {
			t.Errorf("expected util.go line %d to be valid", ln)
		}
	}

	// A line not in the diff should be absent
	if mainLines[999] {
		t.Error("line 999 should not be valid in main.go")
	}
}

func TestParseDiffLines_Empty(t *testing.T) {
	validLines := parseDiffLines("")
	if len(validLines) != 0 {
		t.Errorf("expected empty map for empty diff, got %v", validLines)
	}
}

func TestFilterValidComments(t *testing.T) {
	validLines := map[string]map[int]bool{
		"main.go": {5: true, 10: true, 15: true},
		"util.go": {3: true},
	}

	comments := []review.InlineComment{
		{Path: "main.go", Line: 5, Body: "valid"},
		{Path: "main.go", Line: 10, Body: "valid"},
		{Path: "main.go", Line: 99, Body: "out of range"},
		{Path: "util.go", Line: 3, Body: "valid"},
		{Path: "util.go", Line: 50, Body: "out of range"},
		{Path: "other.go", Line: 1, Body: "file not in diff"},
	}

	valid, dropped := filterValidComments(comments, validLines)

	if len(valid) != 3 {
		t.Errorf("expected 3 valid comments, got %d", len(valid))
	}
	if len(dropped) != 3 {
		t.Errorf("expected 3 dropped comments, got %d", len(dropped))
	}

	for _, c := range valid {
		if c.Body != "valid" {
			t.Errorf("unexpected comment in valid set: %+v", c)
		}
	}
}

func TestFilterValidComments_AllValid(t *testing.T) {
	validLines := map[string]map[int]bool{
		"main.go": {1: true, 2: true},
	}
	comments := []review.InlineComment{
		{Path: "main.go", Line: 1, Body: "a"},
		{Path: "main.go", Line: 2, Body: "b"},
	}
	valid, dropped := filterValidComments(comments, validLines)
	if len(valid) != 2 || len(dropped) != 0 {
		t.Errorf("expected 2 valid and 0 dropped, got %d valid and %d dropped", len(valid), len(dropped))
	}
}

func TestFilterValidComments_NoneValid(t *testing.T) {
	validLines := map[string]map[int]bool{}
	comments := []review.InlineComment{
		{Path: "main.go", Line: 1, Body: "a"},
	}
	valid, dropped := filterValidComments(comments, validLines)
	if len(valid) != 0 || len(dropped) != 1 {
		t.Errorf("expected 0 valid and 1 dropped, got %d valid and %d dropped", len(valid), len(dropped))
	}
}

func TestCreatePendingReview_DropsInvalidLines(t *testing.T) {
	mux := http.NewServeMux()

	var capturedBody map[string]any
	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		rev := &gh.PullRequestReview{
			ID:    gh.Ptr(int64(99)),
			State: gh.Ptr("PENDING"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rev)
	})

	client := testClient(t, mux)

	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+
 func main() {}
`

	result := &review.ReviewResult{
		Summary: "Summary",
		Verdict: "COMMENT",
		Comments: []review.InlineComment{
			{Path: "main.go", Line: 2, Body: "valid line", Severity: "nit"},
			{Path: "main.go", Line: 999, Body: "invalid line", Severity: "issue"},
		},
	}

	reviewID, err := client.CreatePendingReview(context.Background(), "owner", "repo", 1, result, diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewID != 99 {
		t.Errorf("expected review ID 99, got %d", reviewID)
	}

	// The review body should include a note about dropped comments.
	body, _ := capturedBody["body"].(string)
	if !strings.Contains(body, "1 comment(s) were dropped") {
		t.Errorf("expected drop note in body, got: %q", body)
	}

	// Only 1 comment should have been sent.
	rawComments, _ := capturedBody["comments"].([]any)
	if len(rawComments) != 1 {
		t.Errorf("expected 1 comment sent to GitHub, got %d", len(rawComments))
	}
}

