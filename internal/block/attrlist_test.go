package block

import (
	"slices"
	"testing"

	"github.com/ypfaff/adocfmt/internal/tree"
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

// TestBlockRoles pins the roles Asciidoctor 2.0.26 gives a block, checked
// against the classes it renders.
func TestBlockRoles(t *testing.T) {
	tests := []struct {
		lines []string
		want  []string
	}{
		{[]string{"[.pre-wrap]"}, []string{"pre-wrap"}},
		{[]string{"[.lead.pre-wrap]"}, []string{"lead", "pre-wrap"}},
		{[]string{"[quote#id.pre-wrap%step]"}, []string{"pre-wrap"}},
		{[]string{`[".pre-wrap"]`}, []string{"pre-wrap"}},
		{[]string{"[role=pre-wrap]"}, []string{"pre-wrap"}},
		{[]string{"[role='pre-wrap']"}, []string{"pre-wrap"}},
		{[]string{"[role= pre-wrap ]"}, []string{"pre-wrap"}},
		{[]string{`[role="lead pre-wrap"]`}, []string{"lead", "pre-wrap"}},
		{[]string{"[.pre-wrap,role=lead]"}, []string{"lead", "pre-wrap"}},
		{[]string{"[role=lead]", "[.pre-wrap]"}, []string{"lead", "pre-wrap"}},
		{[]string{"[.pre-wrap]", "[.lead]"}, []string{"pre-wrap", "lead"}},
		{[]string{"[.pre-wrap]", "[role=lead]"}, []string{"lead"}},
		{[]string{"[role=lead,.pre-wrap]"}, []string{"lead"}},
		{[]string{"[.pre wrap]"}, nil},
		{[]string{"[roles=pre-wrap]"}, nil},
	}

	for _, test := range tests {
		var src []byte
		var meta []tree.Meta
		for _, line := range test.lines {
			meta = append(meta, tree.Meta{Kind: tree.MetaAttributes, Lines: tree.Span{Start: len(src), End: len(src) + len(line)}})
			src = append(src, line...)
		}
		if got := blockRoles(src, meta); !slices.Equal(got, test.want) {
			t.Errorf("blockRoles(%q) = %q, want %q", test.lines, got, test.want)
		}
	}
}
