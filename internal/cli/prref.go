// internal/cli/prref.go
package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// parsePRRef parses "owner/repo#123" or a GitHub PR URL into components.
func parsePRRef(ref string) (owner, repo string, number int, err error) {
	// Handle GitHub URLs: https://github.com/owner/repo/pull/123
	if strings.Contains(ref, "github.com/") {
		return parsePRURL(ref)
	}

	// Handle owner/repo#123 format (existing logic)
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid PR reference %q — expected owner/repo#number", ref)
	}

	number, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number in %q: %w", ref, err)
	}

	repoParts := strings.SplitN(parts[0], "/", 2)
	if len(repoParts) != 2 {
		return "", "", 0, fmt.Errorf("invalid repo in %q — expected owner/repo", ref)
	}

	return repoParts[0], repoParts[1], number, nil
}

func parsePRURL(url string) (string, string, int, error) {
	// Strip protocol and host
	// Expected path: /owner/repo/pull/123
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	parts := strings.Split(url, "/")
	// Find "pull" in the path and extract owner/repo/number
	for i, p := range parts {
		if p == "pull" && i >= 2 && i+1 < len(parts) {
			owner := parts[i-2]
			repo := parts[i-1]
			number, err := strconv.Atoi(parts[i+1])
			if err != nil {
				return "", "", 0, fmt.Errorf("invalid PR number in URL %q: %w", url, err)
			}
			return owner, repo, number, nil
		}
	}
	return "", "", 0, fmt.Errorf("could not parse PR URL %q — expected https://github.com/owner/repo/pull/123", url)
}
