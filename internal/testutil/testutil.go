package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	gh "github.com/google/go-github/v68/github"

	proofgh "github.com/chaz8081/proof/internal/github"
)

// NewTestClient creates a GitHub client pointing at a test server.
func NewTestClient(t *testing.T, mux *http.ServeMux) *proofgh.Client {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ghClient := gh.NewClient(nil)
	ghClient.BaseURL, _ = ghClient.BaseURL.Parse(server.URL + "/")

	return proofgh.NewClientFromGH(ghClient)
}

// FixturePath returns the path to a test fixture file.
func FixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

// LoadFixture reads a test fixture file and returns its contents.
func LoadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(FixturePath(t, name))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

// ServeDiff sets up a handler that returns a diff fixture for the given PR.
func ServeDiff(mux *http.ServeMux, owner, repo string, number int, diffContent string) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
			w.Write([]byte(diffContent))
			return
		}
		// Return minimal PR JSON
		pr := &gh.PullRequest{
			Number: gh.Ptr(number),
			Title:  gh.Ptr("Test PR"),
			Body:   gh.Ptr("Test description"),
			User:   &gh.User{Login: gh.Ptr("testuser")},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pr)
	})
}
