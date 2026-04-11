package cli

import (
	"testing"
)

func TestParseSelection(t *testing.T) {
	tests := []struct {
		input   string
		total   int
		want    []int
		wantNil bool // nil = "new only" default
		wantErr bool
	}{
		{"1,3", 4, []int{1, 3}, false, false},
		{"1-4", 4, []int{1, 2, 3, 4}, false, false},
		{"all", 4, []int{1, 2, 3, 4}, false, false},
		{"", 4, nil, true, false},        // default: new only
		{"new", 4, nil, true, false},
		{"5", 4, nil, false, true},       // out of range
		{"abc", 4, nil, false, true},     // invalid
		{"2", 4, []int{2}, false, false},
		{"1,2-3", 4, []int{1, 2, 3}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseSelection(tt.input, tt.total)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			// compare slices
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
