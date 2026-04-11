package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// isInteractive returns true if stdin is a terminal (not a pipe or /dev/null).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

type prDisplayItem struct {
	Index    int
	Owner    string
	Repo     string
	Number   int
	Title    string
	Author   string
	Status   string // "NEW" or "PENDING"
	ReviewID int64  // if PENDING
}

// displayPRList prints the numbered PR list with status tags.
func displayPRList(w io.Writer, items []prDisplayItem) {
	for _, item := range items {
		tag := "[NEW]    "
		extra := ""
		if item.Status == "PENDING" {
			tag = "[PENDING]"
			extra = " — pending review exists"
		}
		fmt.Fprintf(w, "  %d. %s %s/%s#%d — %s (by @%s)%s\n",
			item.Index, tag, item.Owner, item.Repo, item.Number, item.Title, item.Author, extra)
	}
}

// parseSelection parses user input like "1,3", "1-4", "all", or "" (default: new only).
// Returns the selected indices (1-based).
func parseSelection(input string, total int) ([]int, error) {
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" || input == "new" {
		return nil, nil // sentinel: caller should filter to NEW only
	}

	if input == "all" {
		indices := make([]int, total)
		for i := range indices {
			indices[i] = i + 1
		}
		return indices, nil
	}

	var indices []int
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid selection: %q", part)
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid selection: %q", part)
			}
			for i := start; i <= end; i++ {
				indices = append(indices, i)
			}
		} else {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid selection: %q", part)
			}
			indices = append(indices, n)
		}
	}

	// Validate range
	for _, idx := range indices {
		if idx < 1 || idx > total {
			return nil, fmt.Errorf("selection %d out of range (1-%d)", idx, total)
		}
	}

	return indices, nil
}

// promptSelection reads user selection from stdin.
func promptSelection(r io.Reader, w io.Writer, total int) ([]int, error) {
	fmt.Fprintf(w, "\nSelect PRs to review (e.g., 1,3 or 1-4 or all) [default: new only]: ")
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no input received")
	}
	return parseSelection(scanner.Text(), total)
}
