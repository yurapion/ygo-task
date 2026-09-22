package claimcheck

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateKeepsRunesIntact pins the rune boundary. These feeds carry German
// and Spanish prose, and the unparsed block exists to quote text verbatim — a
// byte slice through a multi-byte rune prints mojibake in the one place the
// report is claiming to show the reader exactly what it could not read.
func TestTruncateKeepsRunesIntact(t *testing.T) {
	t.Parallel()

	// A 2-byte rune sits exactly on the cut boundary.
	s := strings.Repeat("a", 41) + "über die Berge und das Tal dahinter"

	got := truncate(s, 43)
	if !utf8.ValidString(got) {
		t.Errorf("truncate produced invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n > 43 {
		t.Errorf("got %d runes, want at most 43", n)
	}
}
