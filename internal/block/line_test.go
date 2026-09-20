package block

import (
	"slices"
	"testing"
)

// TestBlockOptions pins the spellings Asciidoctor 2.0.26 reads as setting an
// option, checked against what it renders.
func TestBlockOptions(t *testing.T) {
	tests := []struct {
		src  string
		want []string
	}{
		{"[%hardbreaks]", []string{"hardbreaks"}},
		{"[quote%hardbreaks]", []string{"hardbreaks"}},
		{"[.lead%hardbreaks]", []string{"hardbreaks"}},
		{"[#id%hardbreaks.lead%step]", []string{"hardbreaks", "step"}},
		{`["%hardbreaks"]`, []string{"hardbreaks"}},
		{`[options="hardbreaks"]`, []string{"hardbreaks"}},
		{"[opts=hardbreaks]", []string{"hardbreaks"}},
		{"[opts= hardbreaks ]", []string{"hardbreaks"}},
		{"[opts='hardbreaks']", []string{"hardbreaks"}},
		{`[opts="foo, hardbreaks"]`, []string{"foo", "hardbreaks"}},
		{"[opts=hardbreaks,x]", []string{"hardbreaks"}},
		{"[options=hardbreaks,role=x]", []string{"hardbreaks"}},
		{`[quote, Don't, opts="x,hardbreaks"]`, []string{"x", "hardbreaks"}},
		{`[title="a, b", opts=hardbreaks]`, []string{"hardbreaks"}},
		{"[hardbreaks-option=x]", []string{"hardbreaks"}},
		{"[hardbreaks-option=]", []string{"hardbreaks"}},
		{"[hardbreaks]", nil},
		{"[quote, %hardbreaks]", nil},
		{"[role=x%hardbreaks]", nil},
		{"[% hardbreaks]", nil},
		{`[options="hardbreaks x"]`, []string{"hardbreaks x"}},
		{`[options=" hardbreaks"]`, []string{" hardbreaks"}},
		{"[source,go]", nil},
	}

	for _, test := range tests {
		if got := blockOptions([]byte(test.src)); !slices.Equal(got, test.want) {
			t.Errorf("blockOptions(%s) = %q, want %q", test.src, got, test.want)
		}
	}
}
