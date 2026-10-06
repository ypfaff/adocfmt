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

// TestLines pins that a line is handed out the way the parser reads it, since
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

// TestLineBelow pins the line a rule reads back against, which is the one the
// source has under the line it rewrites.
func TestLineBelow(t *testing.T) {
	tests := []struct {
		src  string
		at   int
		want string
	}{
		{"a\nb\n", 0, "b"},
		{"- one\ntwo\n", 3, "two"},
		{"a\n", 0, ""},
		{"a", 0, ""},
		{"a\nb", 0, "b"},
		{"a\nb  \n", 0, "b"},
		{"a\r\nb\r\n", 0, "b"},
		{"a\n\nc\n", 0, ""},
	}

	for _, test := range tests {
		if got := string(LineBelow([]byte(test.src), test.at)); got != test.want {
			t.Errorf("LineBelow(%q, %d) = %q, want %q", test.src, test.at, got, test.want)
		}
	}
}

// TestUnderlinesTitle pins the window a rewritten line has to stay out of: a
// line within one character of the line below it turns that line into an
// underline.
func TestUnderlinesTitle(t *testing.T) {
	tests := []struct {
		title, underline string
		want             bool
	}{
		{"Section", "-------", true},
		{"Section", "------", true},
		{"Section", "--------", true},
		{"Section", "-----", false},
		{"* item", "~~~~~~", true},
		{"-   item", "~~~~~~", false},
		{"Section", "", false},
		{"Section", "***", false},
		{"Section", "--=--", false},
		{".Title", "------", false},
		{"----", "----", false},
		{"Ü", "=", true},
	}

	for _, test := range tests {
		if got := UnderlinesTitle([]byte(test.title), []byte(test.underline)); got != test.want {
			t.Errorf("UnderlinesTitle(%q, %q) = %t, want %t", test.title, test.underline, got, test.want)
		}
	}
}
