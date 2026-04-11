package cli

import (
	"strings"
	"testing"
)

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
