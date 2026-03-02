package main

import (
	"testing"
)

// TestParsePortList verifies that parsePortList correctly splits, trims, and
// filters comma-separated port strings under all expected input conditions.
func TestParsePortList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string returns empty slice",
			input: "",
			want:  []string{},
		},
		{
			name:  "single port",
			input: "5020",
			want:  []string{"5020"},
		},
		{
			name:  "multiple ports",
			input: "5040,5041,5042,5043",
			want:  []string{"5040", "5041", "5042", "5043"},
		},
		{
			name:  "whitespace around ports is trimmed",
			input: " 5020 , 5021 , 5022 , 5030 ",
			want:  []string{"5020", "5021", "5022", "5030"},
		},
		{
			name:  "trailing comma is ignored",
			input: "5040,5041,",
			want:  []string{"5040", "5041"},
		},
		{
			name:  "leading comma is ignored",
			input: ",5040,5041",
			want:  []string{"5040", "5041"},
		},
		{
			name:  "adjacent commas produce no empty token",
			input: "5040,,5041",
			want:  []string{"5040", "5041"},
		},
		{
			name:  "water environment default ports",
			input: "5020,5021,5022,5030",
			want:  []string{"5020", "5021", "5022", "5030"},
		},
		{
			name:  "pipeline environment ports",
			input: "5040,5041,5042,5043",
			want:  []string{"5040", "5041", "5042", "5043"},
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable for parallel sub-tests
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parsePortList(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("parsePortList(%q) returned %d elements, want %d: got %v",
					tc.input, len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("parsePortList(%q)[%d] = %q, want %q",
						tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}
