package review

import (
	"strings"
	"testing"
)

func TestParseIgnorePatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty content",
			content:  "",
			expected: nil,
		},
		{
			name:     "only comments and blank lines",
			content:  "# this is a comment\n\n# another comment\n",
			expected: nil,
		},
		{
			name:     "single pattern",
			content:  "vendor/",
			expected: []string{"vendor/"},
		},
		{
			name:     "patterns with comments and blank lines",
			content:  "# ignore generated files\n*.pb.go\n\n# ignore vendor\nvendor/\n",
			expected: []string{"*.pb.go", "vendor/"},
		},
		{
			name:     "inline comment not treated as comment",
			content:  "*.pb.go # not a comment",
			expected: []string{"*.pb.go # not a comment"},
		},
		{
			name:     "whitespace trimmed from pattern lines",
			content:  "  *.pb.go  \n  vendor/  \n",
			expected: []string{"*.pb.go", "vendor/"},
		},
		{
			name:     "mixed patterns",
			content:  "# Generated\n*.pb.go\n*.gen.go\n\n# Vendor\nvendor/\nthird_party/\n",
			expected: []string{"*.pb.go", "*.gen.go", "vendor/", "third_party/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseIgnorePatterns(tt.content)
			if len(got) != len(tt.expected) {
				t.Fatalf("ParseIgnorePatterns() returned %d patterns, want %d; got %v", len(got), len(tt.expected), got)
			}
			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Errorf("pattern[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		want     bool
	}{
		{
			name:     "no patterns",
			path:     "internal/foo.go",
			patterns: nil,
			want:     false,
		},
		{
			name:     "glob match on full path",
			path:     "internal/foo.pb.go",
			patterns: []string{"internal/*.pb.go"},
			want:     true,
		},
		{
			name:     "glob match on basename only",
			path:     "some/deep/path/foo.pb.go",
			patterns: []string{"*.pb.go"},
			want:     true,
		},
		{
			name:     "directory prefix match",
			path:     "vendor/github.com/foo/bar.go",
			patterns: []string{"vendor/"},
			want:     true,
		},
		{
			name:     "directory prefix no match without trailing slash",
			path:     "vendor/github.com/foo/bar.go",
			patterns: []string{"vendor"},
			want:     false,
		},
		{
			name:     "no match",
			path:     "internal/real_code.go",
			patterns: []string{"*.pb.go", "vendor/"},
			want:     false,
		},
		{
			name:     "exact filename match",
			path:     "go.sum",
			patterns: []string{"go.sum"},
			want:     true,
		},
		{
			name:     "basename match for nested file",
			path:     "dir/subdir/go.sum",
			patterns: []string{"go.sum"},
			want:     true,
		},
		{
			name:     "multiple patterns first matches",
			path:     "vendor/foo.go",
			patterns: []string{"vendor/", "*.pb.go"},
			want:     true,
		},
		{
			name:     "multiple patterns second matches",
			path:     "internal/foo.pb.go",
			patterns: []string{"vendor/", "*.pb.go"},
			want:     true,
		},
		{
			name:     "gen file in subdirectory matched by basename glob",
			path:     "proto/api/service.pb.go",
			patterns: []string{"*.pb.go"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIgnore(tt.path, tt.patterns)
			if got != tt.want {
				t.Errorf("ShouldIgnore(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestFilterFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		patterns []string
		want     []string
	}{
		{
			name:     "no patterns returns all files",
			files:    []string{"foo.go", "bar.go"},
			patterns: nil,
			want:     []string{"foo.go", "bar.go"},
		},
		{
			name:     "filters matched files",
			files:    []string{"internal/foo.go", "vendor/bar.go", "internal/baz.pb.go"},
			patterns: []string{"vendor/", "*.pb.go"},
			want:     []string{"internal/foo.go"},
		},
		{
			name:     "no files match patterns",
			files:    []string{"internal/foo.go", "cmd/main.go"},
			patterns: []string{"vendor/", "*.pb.go"},
			want:     []string{"internal/foo.go", "cmd/main.go"},
		},
		{
			name:     "all files matched",
			files:    []string{"vendor/foo.go", "vendor/bar.go"},
			patterns: []string{"vendor/"},
			want:     nil,
		},
		{
			name:     "empty file list",
			files:    []string{},
			patterns: []string{"vendor/"},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterFiles(tt.files, tt.patterns)
			if len(got) != len(tt.want) {
				t.Fatalf("FilterFiles() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("FilterFiles()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilterDiff(t *testing.T) {
	sampleDiff := strings.Join([]string{
		"diff --git a/internal/foo.go b/internal/foo.go",
		"index abc..def 100644",
		"--- a/internal/foo.go",
		"+++ b/internal/foo.go",
		"@@ -1,3 +1,4 @@",
		" package foo",
		"+// added comment",
		" ",
		"diff --git a/vendor/bar.go b/vendor/bar.go",
		"index 111..222 100644",
		"--- a/vendor/bar.go",
		"+++ b/vendor/bar.go",
		"@@ -1,2 +1,3 @@",
		" package bar",
		"+// vendor change",
		"diff --git a/proto/baz.pb.go b/proto/baz.pb.go",
		"index 333..444 100644",
		"--- a/proto/baz.pb.go",
		"+++ b/proto/baz.pb.go",
		"@@ -1,2 +1,3 @@",
		" package proto",
		"+// generated",
		"",
	}, "\n")

	t.Run("no patterns returns full diff", func(t *testing.T) {
		got := FilterDiff(sampleDiff, nil)
		if got != sampleDiff {
			t.Errorf("FilterDiff with no patterns should return diff unchanged")
		}
	})

	t.Run("filters vendor hunk", func(t *testing.T) {
		got := FilterDiff(sampleDiff, []string{"vendor/"})
		if strings.Contains(got, "vendor/bar.go") {
			t.Error("expected vendor/bar.go hunk to be removed")
		}
		if !strings.Contains(got, "internal/foo.go") {
			t.Error("expected internal/foo.go hunk to remain")
		}
		if !strings.Contains(got, "proto/baz.pb.go") {
			t.Error("expected proto/baz.pb.go hunk to remain when only vendor/ pattern set")
		}
	})

	t.Run("filters pb.go hunk by basename glob", func(t *testing.T) {
		got := FilterDiff(sampleDiff, []string{"*.pb.go"})
		if strings.Contains(got, "proto/baz.pb.go") {
			t.Error("expected proto/baz.pb.go hunk to be removed")
		}
		if !strings.Contains(got, "internal/foo.go") {
			t.Error("expected internal/foo.go hunk to remain")
		}
		if !strings.Contains(got, "vendor/bar.go") {
			t.Error("expected vendor/bar.go hunk to remain when only *.pb.go pattern set")
		}
	})

	t.Run("filters multiple hunks", func(t *testing.T) {
		got := FilterDiff(sampleDiff, []string{"vendor/", "*.pb.go"})
		if strings.Contains(got, "vendor/bar.go") {
			t.Error("expected vendor/bar.go to be removed")
		}
		if strings.Contains(got, "proto/baz.pb.go") {
			t.Error("expected proto/baz.pb.go to be removed")
		}
		if !strings.Contains(got, "internal/foo.go") {
			t.Error("expected internal/foo.go to remain")
		}
	})

	t.Run("empty diff returns empty-ish result", func(t *testing.T) {
		got := FilterDiff("", []string{"vendor/"})
		// An empty diff with one split produces [""] which gets a trailing newline appended
		if strings.TrimSpace(got) != "" {
			t.Errorf("expected effectively empty output for empty diff, got %q", got)
		}
	})
}
