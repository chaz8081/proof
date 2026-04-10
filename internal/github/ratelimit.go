package github

import (
	"context"
	"fmt"
	"time"
)

// RateLimitInfo holds GitHub API rate limit details.
type RateLimitInfo struct {
	Remaining int
	Limit     int
	Reset     time.Time
}

// CheckRateLimit queries the GitHub API rate limit for the authenticated user.
func (c *Client) CheckRateLimit(ctx context.Context) (*RateLimitInfo, error) {
	rate, _, err := c.gh.RateLimit.Get(ctx)
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
func (c *Client) WaitForRateLimit(ctx context.Context, threshold int) error {
	info, err := c.CheckRateLimit(ctx)
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
