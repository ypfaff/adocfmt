package tree

import "testing"

// TestDelimiterLine pins that a fence is built from its two chars, so the table
// form comes out of the same code as every other one.
func TestDelimiterLine(t *testing.T) {
	tests := []struct {
		char, fill byte
		width      int
		want       string
	}{
		{'-', '-', 4, "----"},
		{'-', '-', 6, "------"},
		{'.', '.', 5, "....."},
		{'`', '`', 3, "```"},
		{'|', '=', 4, "|==="},
		{'|', '=', 8, "|======="},
		{',', '=', 4, ",==="},
	}

	for _, test := range tests {
		d := Delimiter{Char: test.char, Fill: test.fill, Width: test.width}
		if got := string(d.Line(test.width)); got != test.want {
			t.Errorf("Delimiter{%q, %q}.Line(%d) = %q, want %q",
				test.char, test.fill, test.width, got, test.want)
		}
	}
}
