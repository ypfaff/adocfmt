package block

import (
	"slices"
	"testing"
)

// TestTrimTrailing pins what the trailing whitespace rule removes and what it
// has to leave: the terminator, the indentation, and the space of a hard line
// break.
func TestTrimTrailing(t *testing.T) {
	tests := []struct {
		src, want string
	}{
		{"", ""},
		{"Text.  \n", "Text.\n"},
		{"Text.\t\t\n", "Text.\n"},
		{"Text. \t \n", "Text.\n"},
		{"   \n", "\n"},
		{"\t\n", "\n"},
		{"line + \n", "line +\n"},
		{"  indented  \n", "  indented\n"},
		{"a  \r\nb\t\r\n", "a\r\nb\r\n"},
		{"  \r\n", "\r\n"},
		{"no newline  ", "no newline"},
		{"clean\n\nlines\n", "clean\n\nlines\n"},
	}

	for _, test := range tests {
		if got := string(TrimTrailing([]byte(test.src))); got != test.want {
			t.Errorf("TrimTrailing(%q) = %q, want %q", test.src, got, test.want)
		}
	}
}

// TestLines pins that a line is handed out the way the scanner reads it, since
// a delimiter followed by trailing whitespace still closes its block.
func TestLines(t *testing.T) {
	tests := []struct {
		src  string
		want []string
	}{
		{"", nil},
		{"one\n", []string{"one"}},
		{"one\ntwo\n", []string{"one", "two"}},
		{"no newline", []string{"no newline"}},
		{"one\r\ntwo\r\n", []string{"one", "two"}},
		{"----  \n", []string{"----"}},
		{"a\n\nb\n", []string{"a", "", "b"}},
		{"  indented\n", []string{"  indented"}},
	}

	for _, test := range tests {
		var got []string
		for _, line := range Lines([]byte(test.src)) {
			got = append(got, string(line))
		}
		if !slices.Equal(got, test.want) {
			t.Errorf("Lines(%q) = %q, want %q", test.src, got, test.want)
		}
	}
}

// TestDelimiterShape pins the two chars a fence is built from. A table fence
// fills out with = rather than with the char it opens on, which is the one
// place a fence is not a run of a single char.
func TestDelimiterShape(t *testing.T) {
	tests := []struct {
		src        string
		char, fill byte
		width      int
	}{
		{"--", '-', '-', 2},
		{"```", '`', '`', 3},
		{"```go", '`', '`', 3},
		{"----", '-', '-', 4},
		{"......", '.', '.', 6},
		{"|=======", '|', '=', 8},
		{",===", ',', '=', 4},
		{":===", ':', '=', 4},
		{"!===", '!', '=', 4},
	}

	for _, test := range tests {
		sh, ok := delimiterShape([]byte(test.src))
		if !ok || sh.char != test.char || sh.fill != test.fill || sh.width != test.width {
			t.Errorf("delimiterShape(%q) = %q %q %d, %t, want %q %q %d, true",
				test.src, sh.char, sh.fill, sh.width, ok, test.char, test.fill, test.width)
		}
	}
}
