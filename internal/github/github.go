// internal/github/github.go
package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// Client wraps the go-github client with proof-specific functionality.
type Client struct {
	gh *gh.Client
}

// NewClient creates a GitHub API client authenticated with the given token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{gh: gh.NewClient(tc)}
}

// PRInfo is a lightweight representation of a PR needing review.
type PRInfo struct {
	Owner  string
	Repo   string
	Number int
	Title  string
	Author string
	Draft  bool
}

// FindOptions controls filtering behavior for FindReviewRequests.
type FindOptions struct {
	IgnoreDrafts bool
	IgnoreWIP    bool
}

// FindOption is a functional option for FindReviewRequests.
type FindOption func(*FindOptions)

// WithIgnoreDrafts filters out draft PRs from search results.
func WithIgnoreDrafts(v bool) FindOption {
	return func(o *FindOptions) { o.IgnoreDrafts = v }
}

// WithIgnoreWIP filters out PRs with "wip" in the title.
func WithIgnoreWIP(v bool) FindOption {
	return func(o *FindOptions) { o.IgnoreWIP = v }
}

// FindReviewRequests searches for open PRs where the authenticated user
// is a requested reviewer, filtered to the given repos.
func (c *Client) FindReviewRequests(ctx context.Context, repos []string, opts ...FindOption) ([]PRInfo, error) {
	options := &FindOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := "is:open is:pr review-requested:@me"
	if options.IgnoreDrafts {
		query += " draft:false"
	}

	// Build repo filter
	if len(repos) > 0 {
		var repoFilters []string
		for _, r := range repos {
			if strings.HasSuffix(r, "/*") {
				org := strings.TrimSuffix(r, "/*")
				repoFilters = append(repoFilters, fmt.Sprintf("org:%s", org))
			} else {
				repoFilters = append(repoFilters, fmt.Sprintf("repo:%s", r))
			}
		}
		query += " " + strings.Join(repoFilters, " ")
	}

	result, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
		Sort:  "updated",
		Order: "desc",
	})
	if err != nil {
		return nil, fmt.Errorf("searching for review requests: %w", err)
	}

	var prs []PRInfo
	for _, issue := range result.Issues {
		owner, repo := parseRepoURL(issue.GetRepositoryURL())
		pr := PRInfo{
			Owner:  owner,
			Repo:   repo,
			Number: issue.GetNumber(),
			Title:  issue.GetTitle(),
			Author: issue.GetUser().GetLogin(),
			Draft:  issue.GetDraft(),
		}

		if options.IgnoreWIP && strings.Contains(strings.ToLower(pr.Title), "wip") {
			continue
		}

		prs = append(prs, pr)
	}

	return prs, nil
}

// parseRepoURL extracts owner/repo from a GitHub API repository URL.
// e.g., "https://api.github.com/repos/owner/repo" -> "owner", "repo"
func parseRepoURL(url string) (string, string) {
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}
