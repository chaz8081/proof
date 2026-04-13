package cli

import (
	"strings"
	"testing"
)

func TestParseIndex(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  int
	}{
		{"1\n", 3, 0},   // 1-based input "1" → index 0
		{"2\n", 3, 1},   // 1-based input "2" → index 1
		{"3\n", 3, 2},   // 1-based input "3" → index 2
		{"\n", 3, 0},    // empty → default 0
		{"0\n", 3, 0},   // out-of-range (too low) → default 0
		{"4\n", 3, 0},   // out-of-range (too high) → default 0
		{"abc\n", 3, 0}, // non-numeric → default 0
		{"2\n", 1, 0},   // out-of-range for max=1 → default 0
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseIndex(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("parseIndex(%q, %d) = %d, want %d", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		input      string
		defaultVal bool
		want       bool
	}{
		{"y\n", false, true},
		{"yes\n", false, true},
		{"Y\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
		{"\n", true, true},        // empty = default
		{"\n", false, false},      // empty = default (false)
		{"maybe\n", false, false}, // unknown = default
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := promptYesNo(strings.NewReader(tt.input), tt.defaultVal)
			if got != tt.want {
				t.Errorf("promptYesNo(%q, %v) = %v, want %v", tt.input, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a, b, c\n", []string{"a", "b", "c"}},
		{"owner/repo\n", []string{"owner/repo"}},
		{"\n", nil},
		{"a,,b\n", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseCSV(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
