package block

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

// TestDelimiterFences pins which lines a fence may not be shortened onto, which
// is what limits how far the delimiter rule may go.
func TestDelimiterFences(t *testing.T) {
	listing := Delimiter{Char: '-', Fill: '-', Width: 6}
	table := Delimiter{Char: '|', Fill: '=', Width: 8}
	fenced := Delimiter{Char: '`', Fill: '`', Width: 3}

	tests := []struct {
		delim Delimiter
		line  string
		width int
		ok    bool
	}{
		{listing, "----", 4, true},
		{listing, "------", 6, true},
		{listing, "", 0, false},
		{listing, "-", 0, false},
		{listing, "code", 0, false},
		{listing, "----x", 0, false},
		{listing, "====", 0, false},
		{table, "|===", 4, true},
		{table, "|=======", 8, true},
		{table, "| a | b", 0, false},
		{table, ",===", 0, false},
		{table, "====", 0, false},
		{fenced, "```", 3, true},
		{fenced, "```go", 0, false},
	}

	for _, test := range tests {
		width, ok := test.delim.Fences([]byte(test.line))
		if width != test.width || ok != test.ok {
			t.Errorf("Delimiter{%q, %q}.Fences(%q) = %d, %t, want %d, %t",
				test.delim.Char, test.delim.Fill, test.line, width, ok, test.width, test.ok)
		}
	}
}
