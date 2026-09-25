package printer

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/sentence"
)

// reflowed returns the lines of a paragraph or an admonition the way the
// sentence rule writes them: one sentence per line, or as they stand where the
// scanner would read that as something else.
func reflowed(src []byte, node block.Node, eol string) []byte {
	lines := src[node.Lines().Start:node.Lines().End]
	if node.Frozen() {
		return lines
	}
	built := sentence.Reflow(block.Lines(lines))
	out := append(bytes.Join(built, []byte(eol)), block.Terminator(lines)...)
	if !block.ReadsAs(node, out) {
		return lines
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
