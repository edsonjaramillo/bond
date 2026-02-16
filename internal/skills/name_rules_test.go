package skills

import (
	"strings"
	"testing"
)

func TestCheckSkillName(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantRuneCount     int
		wantEmpty         bool
		wantTooLong       bool
		wantInvalidFormat bool
	}{
		{
			name:              "empty",
			input:             "",
			wantRuneCount:     0,
			wantEmpty:         true,
			wantTooLong:       false,
			wantInvalidFormat: false,
		},
		{
			name:              "valid",
			input:             "go-api",
			wantRuneCount:     6,
			wantEmpty:         false,
			wantTooLong:       false,
			wantInvalidFormat: false,
		},
		{
			name:              "too long",
			input:             strings.Repeat("a", SkillNameMaxRunes+1),
			wantRuneCount:     SkillNameMaxRunes + 1,
			wantEmpty:         false,
			wantTooLong:       true,
			wantInvalidFormat: false,
		},
		{
			name:              "invalid format",
			input:             "Go",
			wantRuneCount:     2,
			wantEmpty:         false,
			wantTooLong:       false,
			wantInvalidFormat: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckSkillName(tc.input)
			if got.RuneCount != tc.wantRuneCount {
				t.Fatalf("RuneCount = %d, want %d", got.RuneCount, tc.wantRuneCount)
			}
			if got.Empty != tc.wantEmpty {
				t.Fatalf("Empty = %v, want %v", got.Empty, tc.wantEmpty)
			}
			if got.TooLong != tc.wantTooLong {
				t.Fatalf("TooLong = %v, want %v", got.TooLong, tc.wantTooLong)
			}
			if got.InvalidFormat != tc.wantInvalidFormat {
				t.Fatalf("InvalidFormat = %v, want %v", got.InvalidFormat, tc.wantInvalidFormat)
			}
		})
	}
}
