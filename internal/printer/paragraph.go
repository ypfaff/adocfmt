package printer

import (
	"bytes"
	"reflect"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/sentence"
)

// reflowed returns the lines of a paragraph or an admonition the way the
// sentence rule writes them: one sentence per line, or as they stand.
//
// A join or a split can turn text into syntax: a line starting like a list
// item, a label that now opens an admonition, a first line that now reads as
// an attribute line. The built node is therefore read back, and one the scanner
// no longer reads as the same kind of node stays as it stands. The read back
// sees the node alone, without the lists open around it, so a line that would
// start an item of one of them is refused on its own.
func reflowed(src []byte, node block.Node, eol string) []byte {
	lines := src[node.Lines().Start:node.Lines().End]
	if node.Frozen() {
		return lines
	}
	built := sentence.Reflow(block.Lines(lines))
	out := append(bytes.Join(built, []byte(eol)), block.Terminator(lines)...)

	doc, err := block.Scan(out)
	if err != nil || len(doc.Nodes) != 1 || doc.Nodes[0].Frozen() ||
		reflect.TypeOf(doc.Nodes[0]) != reflect.TypeOf(node) {
		return lines
	}
	for _, line := range built[1:] {
		if _, ok := block.ListMarker(line); ok {
			return lines
		}
	}
	return out
}

func (p *printer) prose(node block.Node) {
	if p.raw {
		p.span(node.Lines())
		return
	}
	p.out.Write(block.TrimTrailing(reflowed(p.src, node, p.eol)))
}
