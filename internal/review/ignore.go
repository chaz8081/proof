package review

import (
	"bufio"
	"path/filepath"
	"strings"
)

// ParseIgnorePatterns reads a .proofignore file content and returns patterns.
func ParseIgnorePatterns(content string) []string {
	var patterns []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// ShouldIgnore returns true if the file path matches any ignore pattern.
func ShouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Match against full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Match against filename only (for patterns like "*.pb.go")
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Match against directory prefix (for patterns like "vendor/")
		if strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, pattern) {
			return true
		}
	}
	return false
}

// FilterFiles removes ignored files from the list.
func FilterFiles(files []string, patterns []string) []string {
	if len(patterns) == 0 {
		return files
	}
	var filtered []string
	for _, f := range files {
		if !ShouldIgnore(f, patterns) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// FilterDiff removes hunks for ignored files from a unified diff.
func FilterDiff(diff string, patterns []string) string {
	if len(patterns) == 0 {
		return diff
	}
	var result strings.Builder
	lines := strings.Split(diff, "\n")
	skip := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			// Extract filename: "diff --git a/path b/path"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				path := strings.TrimPrefix(parts[3], "b/")
				skip = ShouldIgnore(path, patterns)
			}
		}
		if !skip {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	return result.String()
}
