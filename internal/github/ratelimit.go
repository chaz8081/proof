package github

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"
)

// RateLimitInfo holds GitHub API rate limit details.
type RateLimitInfo struct {
	Remaining int
	Limit     int
	Reset     time.Time
}

// CheckRateLimit queries the GitHub API rate limit for the authenticated user.
func (c *Client) CheckRateLimit() (*RateLimitInfo, error) {
	rate, _, err := c.gh.RateLimit.Get(nil)
	if err != nil {
		return nil, err
	}

	return &RateLimitInfo{
		Remaining: rate.Core.Remaining,
		Limit:     rate.Core.Limit,
		Reset:     rate.Core.Reset.Time,
	}, nil
}

// WaitForRateLimit blocks until the rate limit resets if remaining is below threshold.
func (c *Client) WaitForRateLimit(threshold int) error {
	info, err := c.CheckRateLimit()
	if err != nil {
		return err
	}

	if info.Remaining < threshold {
		waitTime := time.Until(info.Reset)
		fmt.Printf("Rate limited. Waiting %v for reset...\n", waitTime)
		time.Sleep(waitTime)
	}

	return nil
}

// GetTokenFromGHCLI shells out to the gh CLI to get the current auth token.
func GetTokenFromGHCLI() (string, error) {
	cmd := exec.Command("sh", "-c", "gh auth token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %v", err)
	}
	return string(output), nil
}

// ParsePRURL extracts owner, repo, and PR number from a GitHub PR URL.
func ParsePRURL(url string) (string, string, int, error) {
	// Quick and dirty parsing: https://github.com/owner/repo/pull/123
	parts := splitURL(url)
	if len(parts) < 5 {
		return "", "", 0, fmt.Errorf("invalid PR URL")
	}

	num, _ := strconv.Atoi(parts[len(parts)-1])
	owner := parts[len(parts)-4]
	repo := parts[len(parts)-3]
	return owner, repo, num, nil
}

func splitURL(url string) []string {
	var parts []string
	current := ""
	for i := 0; i < len(url); i++ {
		if url[i] == '/' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(url[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// FormatReviewComment builds a review comment string from components.
func FormatReviewComment(severity, message, suggestion string) string {
	result := fmt.Sprintf("[%s] %s", severity, message)
	if suggestion != "" {
		result = result + "\n```suggestion\n" + suggestion + "\n```"
	}
	return result
}

// RetryWithBackoff retries a function with exponential backoff.
func RetryWithBackoff(fn func() error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(i*i) * time.Second)
	}
	return err
}

// ParseHTTPResponse extracts rate limit headers from an HTTP response.
func ParseHTTPResponse(resp *http.Response) *RateLimitInfo {
	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	reset, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)

	return &RateLimitInfo{
		Remaining: remaining,
		Limit:     limit,
		Reset:     time.Unix(reset, 0),
	}
}
