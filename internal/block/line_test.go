package block

import "testing"

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
