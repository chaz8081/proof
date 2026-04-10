// internal/cli/submit_test.go
package cli

import (
	"testing"
)

func TestValidateVerdict(t *testing.T) {
	tests := []struct {
		verdict string
		wantErr bool
	}{
		{"APPROVE", false},
		{"REQUEST_CHANGES", false},
		{"COMMENT", false},
		{"approve", true},
		{"LGTM", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.verdict, func(t *testing.T) {
			err := validateVerdict(tt.verdict)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for verdict %q, got nil", tt.verdict)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for verdict %q: %v", tt.verdict, err)
			}
		})
	}
}

func TestResolveVerdict(t *testing.T) {
	tests := []struct {
		name           string
		approve        bool
		requestChanges bool
		verdict        string
		wantVerdict    string
		wantErr        bool
	}{
		{
			name:        "--approve sets verdict to APPROVE",
			approve:     true,
			wantVerdict: "APPROVE",
		},
		{
			name:           "--request-changes sets verdict to REQUEST_CHANGES",
			requestChanges: true,
			wantVerdict:    "REQUEST_CHANGES",
		},
		{
			name:        "--verdict passes through unchanged",
			verdict:     "COMMENT",
			wantVerdict: "COMMENT",
		},
		{
			name:        "no flags returns empty string",
			wantVerdict: "",
		},
		{
			name:           "--approve and --request-changes together returns error",
			approve:        true,
			requestChanges: true,
			wantErr:        true,
		},
		{
			name:    "--approve and --verdict together returns error",
			approve: true,
			verdict: "COMMENT",
			wantErr: true,
		},
		{
			name:           "--request-changes and --verdict together returns error",
			requestChanges: true,
			verdict:        "COMMENT",
			wantErr:        true,
		},
		{
			name:           "all three set returns error",
			approve:        true,
			requestChanges: true,
			verdict:        "COMMENT",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveVerdict(tt.approve, tt.requestChanges, tt.verdict)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantVerdict {
				t.Errorf("got %q, want %q", got, tt.wantVerdict)
			}
		})
	}
}

func TestParsePRRef(t *testing.T) {
	tests := []struct {
		input      string
		wantOwner  string
		wantRepo   string
		wantNumber int
		wantErr    bool
	}{
		{"owner/repo#123", "owner", "repo", 123, false},
		{"my-org/my-repo#1", "my-org", "my-repo", 1, false},
		{"bad-format", "", "", 0, true},
		{"owner/repo#abc", "", "", 0, true},
		{"noslash#123", "", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, number, err := parsePRRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo || number != tt.wantNumber {
				t.Errorf("got (%s, %s, %d), want (%s, %s, %d)",
					owner, repo, number, tt.wantOwner, tt.wantRepo, tt.wantNumber)
			}
		})
	}
}
