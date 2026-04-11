package cli

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"7 days", "7d", 7 * 24 * time.Hour, false},
		{"30 days", "30d", 30 * 24 * time.Hour, false},
		{"24 hours", "24h", 24 * time.Hour, false},
		{"1 hour", "1h", time.Hour, false},
		{"90 minutes", "90m", 90 * time.Minute, false},
		{"too short", "7", 0, true},
		{"empty string", "", 0, true},
		{"non-numeric value", "xd", 0, true},
		{"unknown unit falls through to stdlib", "1s", time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
