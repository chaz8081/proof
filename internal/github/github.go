// internal/github/github.go
package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaz8081/proof/internal/review"
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
	Teams        []string
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

// WithTeams adds team-review-requested queries for the given teams.
func WithTeams(teams []string) FindOption {
	return func(o *FindOptions) { o.Teams = teams }
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
		Sort:        "updated",
		Order:       "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
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

// GetPRContext fetches all context needed for AI review: PR metadata, diff, and file list.
func (c *Client) GetPRContext(ctx context.Context, owner, repo string, number int) (*review.PRContext, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("getting PR: %w", err)
	}

	diff, _, err := c.gh.PullRequests.GetRaw(ctx, owner, repo, number, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return nil, fmt.Errorf("getting diff: %w", err)
	}

	commitFiles, _, err := c.gh.PullRequests.ListFiles(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}

	files := make([]string, len(commitFiles))
	for i, f := range commitFiles {
		files[i] = f.GetFilename()
	}

	return &review.PRContext{
		Owner:       owner,
		Repo:        repo,
		Number:      number,
		Title:       pr.GetTitle(),
		Description: pr.GetBody(),
		Diff:        diff,
		Files:       files,
	}, nil
}

// PendingReviewInfo represents a pending review on a PR.
type PendingReviewInfo struct {
	ID   int64
	Body string
	User string
}

// CreatePendingReview creates a PENDING review with inline comments.
// The review is only visible to the authenticated user until submitted.
func (c *Client) CreatePendingReview(ctx context.Context, owner, repo string, number int, result *review.ReviewResult) (int64, error) {
	var comments []*gh.DraftReviewComment
	for _, comment := range result.Comments {
		comments = append(comments, &gh.DraftReviewComment{
			Path: gh.Ptr(comment.Path),
			Line: gh.Ptr(comment.Line),
			Side: gh.Ptr("RIGHT"),
			Body: gh.Ptr(comment.FormattedBody()),
		})
	}

	r, _, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, number, &gh.PullRequestReviewRequest{
		Body:     gh.Ptr(result.Summary),
		Comments: comments,
		// Event intentionally omitted — creates a PENDING review
	})
	if err != nil {
		return 0, fmt.Errorf("creating pending review: %w", err)
	}

	return r.GetID(), nil
}

// SubmitReview submits a pending review with the given verdict.
// Valid events: APPROVE, REQUEST_CHANGES, COMMENT
func (c *Client) SubmitReview(ctx context.Context, owner, repo string, number int, reviewID int64, event string) error {
	_, _, err := c.gh.PullRequests.SubmitReview(ctx, owner, repo, number, reviewID, &gh.PullRequestReviewRequest{
		Event: gh.Ptr(event),
	})
	if err != nil {
		return fmt.Errorf("submitting review: %w", err)
	}
	return nil
}

// ListPendingReviews returns all PENDING reviews on a PR.
func (c *Client) ListPendingReviews(ctx context.Context, owner, repo string, number int) ([]PendingReviewInfo, error) {
	reviews, _, err := c.gh.PullRequests.ListReviews(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing reviews: %w", err)
	}

	var pending []PendingReviewInfo
	for _, r := range reviews {
		if r.GetState() == "PENDING" {
			pending = append(pending, PendingReviewInfo{
				ID:   r.GetID(),
				Body: r.GetBody(),
				User: r.GetUser().GetLogin(),
			})
		}
	}
	return pending, nil
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
