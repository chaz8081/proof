// internal/github/github.go
package github

import (
	"context"
	"fmt"
	"strconv"
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

// prKey uniquely identifies a PR for deduplication.
type prKey struct {
	Owner  string
	Repo   string
	Number int
}

// buildRepoFilter constructs the repo/org filter string for a GitHub search query.
func buildRepoFilter(repos []string) string {
	if len(repos) == 0 {
		return ""
	}
	var filters []string
	for _, r := range repos {
		if strings.HasSuffix(r, "/*") {
			org := strings.TrimSuffix(r, "/*")
			filters = append(filters, fmt.Sprintf("org:%s", org))
		} else {
			filters = append(filters, fmt.Sprintf("repo:%s", r))
		}
	}
	return " " + strings.Join(filters, " ")
}

// searchPRs executes a GitHub issue search query and returns PRInfo results,
// applying IgnoreWIP filtering.
func (c *Client) searchPRs(ctx context.Context, query string, options *FindOptions) ([]PRInfo, error) {
	result, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, err
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

// FindReviewRequests searches for open PRs where the authenticated user
// is a requested reviewer, filtered to the given repos.
// If teams are configured, additional queries are run for each team and results
// are deduplicated by owner/repo/number.
func (c *Client) FindReviewRequests(ctx context.Context, repos []string, opts ...FindOption) ([]PRInfo, error) {
	options := &FindOptions{}
	for _, opt := range opts {
		opt(options)
	}

	repoFilter := buildRepoFilter(repos)

	// Query for personal review requests
	query := "is:open is:pr review-requested:@me"
	if options.IgnoreDrafts {
		query += " draft:false"
	}
	query += repoFilter

	prs, err := c.searchPRs(ctx, query, options)
	if err != nil {
		return nil, fmt.Errorf("searching for review requests: %w", err)
	}

	// Run separate queries per team and deduplicate results.
	if len(options.Teams) > 0 {
		// Build a set of already-seen PRs from the personal query.
		seen := make(map[prKey]struct{}, len(prs))
		for _, pr := range prs {
			seen[prKey{pr.Owner, pr.Repo, pr.Number}] = struct{}{}
		}

		for _, team := range options.Teams {
			teamQuery := "is:open is:pr team-review-requested:" + team
			if options.IgnoreDrafts {
				teamQuery += " draft:false"
			}
			teamQuery += repoFilter

			teamPRs, err := c.searchPRs(ctx, teamQuery, options)
			if err != nil {
				return nil, fmt.Errorf("searching for team review requests (%s): %w", team, err)
			}

			for _, pr := range teamPRs {
				k := prKey{pr.Owner, pr.Repo, pr.Number}
				if _, exists := seen[k]; !exists {
					seen[k] = struct{}{}
					prs = append(prs, pr)
				}
			}
		}
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

// parseDiffLines extracts the set of valid new-file line numbers from a unified diff.
// Returns map[filepath]map[lineNumber]true by parsing @@ hunk headers.
func parseDiffLines(diff string) map[string]map[int]bool {
	result := make(map[string]map[int]bool)
	var currentFile string
	var currentLine int

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			// Strip the "b/" prefix that unified diffs add
			path := strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
			currentFile = path
			if _, ok := result[currentFile]; !ok {
				result[currentFile] = make(map[int]bool)
			}
		case strings.HasPrefix(line, "@@ "):
			// Parse @@ -old,count +new,start[,count] @@
			// We need the new-file start line number.
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}
			newRange := strings.TrimPrefix(parts[2], "+")
			startStr := strings.SplitN(newRange, ",", 2)[0]
			start, err := strconv.Atoi(startStr)
			if err != nil {
				continue
			}
			currentLine = start
		case currentFile != "" && len(line) > 0:
			switch line[0] {
			case '+':
				if result[currentFile] != nil {
					result[currentFile][currentLine] = true
				}
				currentLine++
			case ' ':
				// Context line — exists in new file
				if result[currentFile] != nil {
					result[currentFile][currentLine] = true
				}
				currentLine++
			case '-':
				// Removed line — not in new file, don't advance currentLine
			}
		}
	}
	return result
}

// filterValidComments removes comments with line numbers outside the diff hunks.
// Returns valid comments and a slice of dropped comments for reporting.
func filterValidComments(comments []review.InlineComment, validLines map[string]map[int]bool) (valid, dropped []review.InlineComment) {
	for _, c := range comments {
		fileLines, ok := validLines[c.Path]
		if ok && fileLines[c.Line] {
			valid = append(valid, c)
		} else {
			dropped = append(dropped, c)
		}
	}
	return valid, dropped
}

// CreatePendingReview creates a PENDING review with inline comments.
// The review is only visible to the authenticated user until submitted.
// diff is the unified diff for the PR; comments with line numbers outside the
// diff hunks are filtered out and noted in the review body to avoid 422 errors.
func (c *Client) CreatePendingReview(ctx context.Context, owner, repo string, number int, result *review.ReviewResult, diff string) (int64, error) {
	// Filter out comments whose line numbers fall outside the diff.
	validLines := parseDiffLines(diff)
	validComments, droppedComments := filterValidComments(result.Comments, validLines)

	body := result.Summary
	if len(droppedComments) > 0 {
		body += fmt.Sprintf("\n\n_Note: %d comment(s) were dropped because their line numbers fall outside the diff._", len(droppedComments))
	}

	var comments []*gh.DraftReviewComment
	for _, comment := range validComments {
		comments = append(comments, &gh.DraftReviewComment{
			Path: gh.Ptr(comment.Path),
			Line: gh.Ptr(comment.Line),
			Side: gh.Ptr("RIGHT"),
			Body: gh.Ptr(comment.FormattedBody()),
		})
	}

	r, _, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, number, &gh.PullRequestReviewRequest{
		Body:     gh.Ptr(body),
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

// DeletePendingReview deletes a pending review from a PR.
func (c *Client) DeletePendingReview(ctx context.Context, owner, repo string, number int, reviewID int64) error {
	_, _, err := c.gh.PullRequests.DeletePendingReview(ctx, owner, repo, number, reviewID)
	if err != nil {
		return fmt.Errorf("deleting pending review: %w", err)
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
