// internal/cli/submit_test.go
package cli

import "testing"

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
