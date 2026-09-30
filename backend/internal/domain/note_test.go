package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNoteValidate(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		content string
		wantErr string
	}{
		{"valid", "Valid Title", "content", ""},
		{"valid multibyte title", "Привет мир", "content", ""},
		{"valid title at min length", "abc", "content", ""},
		{"valid title at max length", strings.Repeat("a", 100), "content", ""},

		{"empty title", "", "content", "title is required"},
		{"title too short", "ab", "content", "title must be at least 3 characters"},
		{"title too long", strings.Repeat("a", 101), "content", "title must be at most 100 characters"},
		{"empty content", "Valid Title", "", "content is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := &Note{Title: tc.title, Content: tc.content}
			err := n.Validate()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Validate() = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// The limit is expressed in runes so that non-ASCII titles are measured by
// visible characters, not bytes.
func TestNoteValidateCountsRunesNotBytes(t *testing.T) {
	// "Мир" is 3 runes but 6 bytes; a byte-based check would misjudge the limit.
	const multibyte = "Мир"

	if got := len(multibyte); got != 6 {
		t.Fatalf("test fixture assumption broken: len(%q) = %d, want 6", multibyte, got)
	}
	if got := utf8.RuneCountInString(multibyte); got != 3 {
		t.Fatalf("test fixture assumption broken: rune count = %d, want 3", got)
	}

	n := &Note{Title: multibyte, Content: "c"}
	if err := n.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for a 3-rune multibyte title", err)
	}
}
