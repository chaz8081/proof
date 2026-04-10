package testutil_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	gh "github.com/google/go-github/v68/github"

	"github.com/chaz8081/proof/internal/testutil"
)

func TestNewTestClient_MakesRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*gh.PullRequestReview{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	})

	client := testutil.NewTestClient(t, mux)
	reviews, err := client.ListPendingReviews(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 0 {
		t.Errorf("expected 0 pending reviews, got %d", len(reviews))
	}
}

func TestLoadFixture_SampleDiff(t *testing.T) {
	data := testutil.LoadFixture(t, "sample.diff")
	if len(data) == 0 {
		t.Fatal("expected non-empty fixture data")
	}
	content := string(data)
	if len(content) == 0 {
		t.Error("expected non-empty diff content")
	}
}

func TestLoadFixture_PRJSON(t *testing.T) {
	data := testutil.LoadFixture(t, "pr.json")
	var pr map[string]any
	if err := json.Unmarshal(data, &pr); err != nil {
		t.Fatalf("pr.json is not valid JSON: %v", err)
	}
	if pr["number"] == nil {
		t.Error("expected pr.json to contain a number field")
	}
}

func TestLoadFixture_FilesJSON(t *testing.T) {
	data := testutil.LoadFixture(t, "files.json")
	var files []map[string]any
	if err := json.Unmarshal(data, &files); err != nil {
		t.Fatalf("files.json is not valid JSON: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected files.json to contain at least one file")
	}
}

func TestFixturePath(t *testing.T) {
	path := testutil.FixturePath(t, "sample.diff")
	if path == "" {
		t.Error("expected non-empty fixture path")
	}
}

func TestServeDiff(t *testing.T) {
	mux := http.NewServeMux()
	diffContent := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1,2 @@\n package foo\n+// added\n"
	testutil.ServeDiff(mux, "owner", "repo", 7, diffContent)

	// ServeDiff only handles the PR endpoint; register the files endpoint separately.
	mux.HandleFunc("/repos/owner/repo/pulls/7/files", func(w http.ResponseWriter, r *http.Request) {
		files := []*gh.CommitFile{{Filename: gh.Ptr("foo.go")}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	client := testutil.NewTestClient(t, mux)
	prCtx, err := client.GetPRContext(context.Background(), "owner", "repo", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prCtx.Title != "Test PR" {
		t.Errorf("expected title 'Test PR', got %q", prCtx.Title)
	}
	if prCtx.Diff != diffContent {
		t.Errorf("expected diff content, got %q", prCtx.Diff)
	}
}
