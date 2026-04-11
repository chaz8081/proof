// internal/github/github_test.go
package github

import (
	"context"
	"encoding/base64"
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

func TestGetReviewComments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews/42/comments", func(w http.ResponseWriter, r *http.Request) {
		comments := []*gh.PullRequestComment{
			{
				Path: gh.Ptr("main.go"),
				Line: gh.Ptr(10),
				Body: gh.Ptr("[issue] Missing error check"),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	})
	client := testClient(t, mux)
	comments, err := client.GetReviewComments(context.Background(), "owner", "repo", 1, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
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

func TestSearchPRs_Pagination(t *testing.T) {
	mux := http.NewServeMux()

	// Two pages of results: page 1 returns PR #1, page 2 returns PR #2.
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")

		var issues []*gh.Issue
		switch page {
		case "", "1":
			issues = []*gh.Issue{
				{
					Number:           gh.Ptr(1),
					Title:            gh.Ptr("First PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/1")},
					User:             &gh.User{Login: gh.Ptr("alice")},
					Draft:            gh.Ptr(false),
				},
			}
			// Signal that there is a second page.
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
		case "2":
			issues = []*gh.Issue{
				{
					Number:           gh.Ptr(2),
					Title:            gh.Ptr("Second PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/2")},
					User:             &gh.User{Login: gh.Ptr("bob")},
					Draft:            gh.Ptr(false),
				},
			}
			// No Link header — last page.
		}

		result := &gh.IssuesSearchResult{
			Total:  gh.Ptr(2),
			Issues: issues,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	prs, err := client.FindReviewRequests(context.Background(), []string{"owner/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs from paginated results, got %d: %+v", len(prs), prs)
	}

	prNums := make(map[int]bool)
	for _, pr := range prs {
		prNums[pr.Number] = true
	}
	for _, expected := range []int{1, 2} {
		if !prNums[expected] {
			t.Errorf("expected PR #%d in results", expected)
		}
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

// --- helpers for FetchRepoInstructions tests ---

// encodedContent returns the base64-encoded form of s (mimicking GitHub's API encoding).
func encodedContent(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// fileContentJSON builds a minimal GitHub RepositoryContent JSON object with base64 encoding.
func fileContentJSON(name, path, content string) string {
	encoded := encodedContent(content)
	return `{"type":"file","name":"` + name + `","path":"` + path + `","encoding":"base64","content":"` + encoded + `"}`
}

// dirContentJSON builds a minimal GitHub directory listing JSON array.
func dirContentJSON(files []struct{ Name, Path string }) string {
	var items []string
	for _, f := range files {
		items = append(items, `{"type":"file","name":"`+f.Name+`","path":"`+f.Path+`"}`)
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestFetchRepoInstructions_AllPresent(t *testing.T) {
	mux := http.NewServeMux()

	// Serve .github/copilot-instructions.md
	mux.HandleFunc("/repos/owner/repo/contents/.github/copilot-instructions.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fileContentJSON("copilot-instructions.md", ".github/copilot-instructions.md", "Repo-wide review rules.")))
	})

	// Serve .github/instructions directory listing
	mux.HandleFunc("/repos/owner/repo/contents/.github/instructions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(dirContentJSON([]struct{ Name, Path string }{
			{"go.instructions.md", ".github/instructions/go.instructions.md"},
		})))
	})

	// Serve the path-specific file with a matching glob
	mux.HandleFunc("/repos/owner/repo/contents/.github/instructions/go.instructions.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := "---\napplyTo: \"**/*.go\"\n---\nAlways handle errors in Go."
		w.Write([]byte(fileContentJSON("go.instructions.md", ".github/instructions/go.instructions.md", body)))
	})

	// Serve AGENTS.md
	mux.HandleFunc("/repos/owner/repo/contents/AGENTS.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fileContentJSON("AGENTS.md", "AGENTS.md", "Agent instructions here.")))
	})

	client := testClient(t, mux)
	ri, err := client.FetchRepoInstructions(context.Background(), "owner", "repo", []string{"pkg/foo.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ri.RepoWide != "Repo-wide review rules." {
		t.Errorf("unexpected RepoWide: %q", ri.RepoWide)
	}
	if len(ri.PathSpecific) != 1 {
		t.Fatalf("expected 1 path-specific instruction, got %d", len(ri.PathSpecific))
	}
	if !strings.Contains(ri.PathSpecific[0], "Always handle errors") {
		t.Errorf("unexpected PathSpecific content: %q", ri.PathSpecific[0])
	}
	if strings.Contains(ri.PathSpecific[0], "applyTo") {
		t.Error("expected frontmatter stripped from path-specific instruction")
	}
	if ri.AgentInstructions != "Agent instructions here." {
		t.Errorf("unexpected AgentInstructions: %q", ri.AgentInstructions)
	}
}

func TestFetchRepoInstructions_MissingFilesSkipped(t *testing.T) {
	mux := http.NewServeMux()

	// All endpoints return 404
	mux.HandleFunc("/repos/owner/repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	})

	client := testClient(t, mux)
	ri, err := client.FetchRepoInstructions(context.Background(), "owner", "repo", []string{"main.go"})
	if err != nil {
		t.Fatalf("expected no error when files are missing, got: %v", err)
	}
	if ri.RepoWide != "" {
		t.Errorf("expected empty RepoWide, got: %q", ri.RepoWide)
	}
	if len(ri.PathSpecific) != 0 {
		t.Errorf("expected no PathSpecific, got: %v", ri.PathSpecific)
	}
	if ri.AgentInstructions != "" {
		t.Errorf("expected empty AgentInstructions, got: %q", ri.AgentInstructions)
	}
}

func TestFetchRepoInstructions_GlobNoMatch(t *testing.T) {
	mux := http.NewServeMux()

	// copilot-instructions.md missing
	mux.HandleFunc("/repos/owner/repo/contents/.github/copilot-instructions.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	// Directory has a Go-specific instructions file
	mux.HandleFunc("/repos/owner/repo/contents/.github/instructions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(dirContentJSON([]struct{ Name, Path string }{
			{"go.instructions.md", ".github/instructions/go.instructions.md"},
		})))
	})

	// File only applies to *.go but changed files are TypeScript
	mux.HandleFunc("/repos/owner/repo/contents/.github/instructions/go.instructions.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := "---\napplyTo: \"**/*.go\"\n---\nGo-specific instructions."
		w.Write([]byte(fileContentJSON("go.instructions.md", ".github/instructions/go.instructions.md", body)))
	})

	// AGENTS.md missing
	mux.HandleFunc("/repos/owner/repo/contents/AGENTS.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := testClient(t, mux)
	ri, err := client.FetchRepoInstructions(context.Background(), "owner", "repo", []string{"src/app.ts", "src/utils.ts"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ri.PathSpecific) != 0 {
		t.Errorf("expected no path-specific instructions when glob doesn't match, got: %v", ri.PathSpecific)
	}
}

func TestFetchRepoInstructions_NonInstructionFilesSkipped(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/contents/.github/copilot-instructions.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	// Directory contains a mix of files — only .instructions.md files should be used
	mux.HandleFunc("/repos/owner/repo/contents/.github/instructions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(dirContentJSON([]struct{ Name, Path string }{
			{"go.instructions.md", ".github/instructions/go.instructions.md"},
			{"README.md", ".github/instructions/README.md"},
			{"notes.txt", ".github/instructions/notes.txt"},
		})))
	})

	mux.HandleFunc("/repos/owner/repo/contents/.github/instructions/go.instructions.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := "Go instructions without frontmatter."
		w.Write([]byte(fileContentJSON("go.instructions.md", ".github/instructions/go.instructions.md", body)))
	})

	mux.HandleFunc("/repos/owner/repo/contents/AGENTS.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := testClient(t, mux)
	ri, err := client.FetchRepoInstructions(context.Background(), "owner", "repo", []string{"main.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the .instructions.md file should be included (no frontmatter = matches all)
	if len(ri.PathSpecific) != 1 {
		t.Errorf("expected 1 path-specific instruction, got %d", len(ri.PathSpecific))
	}
}

// --- Unit tests for helper functions ---

func TestExtractApplyTo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double-quoted glob",
			input:    "---\napplyTo: \"**/*.go\"\n---\nContent here.",
			expected: "**/*.go",
		},
		{
			name:     "single-quoted glob",
			input:    "---\napplyTo: '**/*.ts'\n---\nContent.",
			expected: "**/*.ts",
		},
		{
			name:     "unquoted glob",
			input:    "---\napplyTo: **/*.go\n---\nContent.",
			expected: "**/*.go",
		},
		{
			name:     "no frontmatter",
			input:    "Just plain content.",
			expected: "",
		},
		{
			name:     "frontmatter without applyTo",
			input:    "---\ntitle: My Instructions\n---\nContent.",
			expected: "",
		},
		{
			name:     "unclosed frontmatter",
			input:    "---\napplyTo: \"**/*.go\"\nContent without closing.",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractApplyTo(tt.input)
			if got != tt.expected {
				t.Errorf("extractApplyTo(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips frontmatter",
			input:    "---\napplyTo: \"**/*.go\"\n---\nActual content here.",
			expected: "Actual content here.",
		},
		{
			name:     "no frontmatter passthrough",
			input:    "Just plain content.",
			expected: "Just plain content.",
		},
		{
			name:     "unclosed frontmatter passthrough",
			input:    "---\napplyTo: test\nNo closing markers.",
			expected: "---\napplyTo: test\nNo closing markers.",
		},
		{
			name:     "trims leading whitespace after frontmatter",
			input:    "---\napplyTo: x\n---\n\n  Content with leading space.",
			expected: "Content with leading space.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.input)
			if got != tt.expected {
				t.Errorf("stripFrontmatter(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMatchesChangedFiles(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		changedFiles []string
		want         bool
	}{
		{
			name:         "no pattern matches all",
			fileContent:  "Instructions without frontmatter.",
			changedFiles: []string{"any/file.go"},
			want:         true,
		},
		{
			name:         "glob matches full path",
			fileContent:  "---\napplyTo: \"**/*.go\"\n---\nContent.",
			changedFiles: []string{"pkg/foo.go"},
			want:         true,
		},
		{
			name:         "glob matches basename",
			fileContent:  "---\napplyTo: \"*.go\"\n---\nContent.",
			changedFiles: []string{"pkg/internal/bar.go"},
			want:         true,
		},
		{
			name:         "glob no match",
			fileContent:  "---\napplyTo: \"**/*.go\"\n---\nContent.",
			changedFiles: []string{"src/app.ts", "src/index.ts"},
			want:         false,
		},
		{
			name:         "empty changed files with pattern",
			fileContent:  "---\napplyTo: \"**/*.go\"\n---\nContent.",
			changedFiles: []string{},
			want:         false,
		},
		{
			name:         "empty changed files without pattern",
			fileContent:  "No frontmatter.",
			changedFiles: []string{},
			want:         true,
		},
		{
			name:         "one of multiple files matches",
			fileContent:  "---\napplyTo: \"**/*.go\"\n---\nContent.",
			changedFiles: []string{"src/app.ts", "pkg/main.go", "README.md"},
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesChangedFiles(tt.fileContent, tt.changedFiles)
			if got != tt.want {
				t.Errorf("matchesChangedFiles() = %v, want %v (pattern=%q, files=%v)",
					got, tt.want, extractApplyTo(tt.fileContent), tt.changedFiles)
			}
		})
	}
}

func TestFetchRepoConfig(t *testing.T) {
	t.Run("valid .proof.yaml", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/owner/repo/contents/.proof.yaml", func(w http.ResponseWriter, r *http.Request) {
			body := "review:\n  instructions: \"Always require tests.\"\n  model: gpt-4o\n"
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fileContentJSON(".proof.yaml", ".proof.yaml", body)))
		})

		client := testClient(t, mux)
		cfg, err := client.FetchRepoConfig(context.Background(), "owner", "repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if cfg.Instructions != "Always require tests." {
			t.Errorf("unexpected Instructions: %q", cfg.Instructions)
		}
		if cfg.Model != "gpt-4o" {
			t.Errorf("unexpected Model: %q", cfg.Model)
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/owner/repo/contents/.proof.yaml", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		})

		client := testClient(t, mux)
		cfg, err := client.FetchRepoConfig(context.Background(), "owner", "repo")
		if err != nil {
			t.Fatalf("expected nil error for missing file, got: %v", err)
		}
		if cfg != nil {
			t.Errorf("expected nil config for missing file, got: %+v", cfg)
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/owner/repo/contents/.proof.yaml", func(w http.ResponseWriter, r *http.Request) {
			// Malformed YAML: tab characters are invalid in YAML
			body := "review:\n\tinstructions: bad\n"
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fileContentJSON(".proof.yaml", ".proof.yaml", body)))
		})

		client := testClient(t, mux)
		cfg, err := client.FetchRepoConfig(context.Background(), "owner", "repo")
		if err == nil {
			t.Errorf("expected error for invalid YAML, got nil (cfg=%+v)", cfg)
		}
		if !strings.Contains(err.Error(), "parsing .proof.yaml") {
			t.Errorf("expected error to mention parsing, got: %v", err)
		}
	})

	t.Run("partial config — only model set", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/repos/owner/repo/contents/.proof.yaml", func(w http.ResponseWriter, r *http.Request) {
			body := "review:\n  model: gpt-4.1\n"
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fileContentJSON(".proof.yaml", ".proof.yaml", body)))
		})

		client := testClient(t, mux)
		cfg, err := client.FetchRepoConfig(context.Background(), "owner", "repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if cfg.Model != "gpt-4.1" {
			t.Errorf("unexpected Model: %q", cfg.Model)
		}
		if cfg.Instructions != "" {
			t.Errorf("expected empty Instructions, got: %q", cfg.Instructions)
		}
	})
}


func TestFindReviewRequests_IncludeOwn(t *testing.T) {
	mux := http.NewServeMux()

	var receivedQueries []string

	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		receivedQueries = append(receivedQueries, q)

		var issues []*gh.Issue

		switch {
		case strings.Contains(q, "review-requested:@me"):
			// Personal review request — PR #1 authored by someone else
			issues = []*gh.Issue{
				{
					Number:           gh.Ptr(1),
					Title:            gh.Ptr("Review Request PR"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/1")},
					User:             &gh.User{Login: gh.Ptr("alice")},
					Draft:            gh.Ptr(false),
				},
				// PR #2 also returned here — will be deduped with the author:@me result
				{
					Number:           gh.Ptr(2),
					Title:            gh.Ptr("Own PR also requested"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/2")},
					User:             &gh.User{Login: gh.Ptr("me")},
					Draft:            gh.Ptr(false),
				},
			}
		case strings.Contains(q, "author:@me"):
			// Own PRs — PR #2 (duplicate) and PR #3 (new)
			issues = []*gh.Issue{
				{
					Number:           gh.Ptr(2),
					Title:            gh.Ptr("Own PR also requested"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/2")},
					User:             &gh.User{Login: gh.Ptr("me")},
					Draft:            gh.Ptr(false),
				},
				{
					Number:           gh.Ptr(3),
					Title:            gh.Ptr("Own PR only"),
					RepositoryURL:    gh.Ptr("https://api.github.com/repos/owner/repo"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/3")},
					User:             &gh.User{Login: gh.Ptr("me")},
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
		WithIncludeOwn(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have exactly 3 unique PRs (PR #2 deduplicated).
	if len(prs) != 3 {
		t.Fatalf("expected 3 PRs after deduplication, got %d: %+v", len(prs), prs)
	}

	// Verify both queries were sent.
	if len(receivedQueries) != 2 {
		t.Fatalf("expected 2 search queries, got %d: %v", len(receivedQueries), receivedQueries)
	}

	hasPersonalQuery := false
	hasOwnQuery := false
	for _, q := range receivedQueries {
		if strings.Contains(q, "review-requested:@me") {
			hasPersonalQuery = true
		}
		if strings.Contains(q, "author:@me") {
			hasOwnQuery = true
		}
	}
	if !hasPersonalQuery {
		t.Errorf("expected a review-requested:@me query, got: %v", receivedQueries)
	}
	if !hasOwnQuery {
		t.Errorf("expected an author:@me query, got: %v", receivedQueries)
	}

	// Verify PR numbers: 1, 2, 3.
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

func TestFindReviewRequests_IncludeOwn_DraftFilter(t *testing.T) {
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
		WithIncludeOwn(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindReviewRequests_IncludeOwn_False(t *testing.T) {
	mux := http.NewServeMux()

	callCount := 0
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		q := r.URL.Query().Get("q")
		if strings.Contains(q, "author:@me") {
			t.Errorf("unexpected author:@me query when IncludeOwn=false: %q", q)
		}
		result := &gh.IssuesSearchResult{Total: gh.Ptr(0), Issues: []*gh.Issue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	_, err := client.FindReviewRequests(
		context.Background(),
		[]string{"owner/repo"},
		WithIncludeOwn(false),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one query (personal review-requested) should be sent.
	if callCount != 1 {
		t.Errorf("expected 1 search query when IncludeOwn=false, got %d", callCount)
	}
}
