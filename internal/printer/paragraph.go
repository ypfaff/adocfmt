package printer

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/sentence"
	"github.com/ypfaff/adocfmt/internal/tree"
)

// reflowed returns the lines of a paragraph or an admonition the way the
// sentence rule writes them: one sentence per line, or as they stand where the
// parser would read that as something else.
func reflowed(src []byte, node tree.Node, eol string) []byte {
	lines := src[node.Base().Lines.Start:node.Base().Lines.End]
	if node.Base().Frozen {
		return lines
	}
	built := sentence.Reflow(block.Lines(lines))
	out := append(bytes.Join(built, []byte(eol)), block.Terminator(lines)...)
	if !block.ReadsAs(node, out) {
		return lines
	}
	return out
}

func (p *printer) prose(node tree.Node) {
	if p.raw {
		p.span(node.Base().Lines)
		return
	}
	p.out.Write(block.TrimTrailing(reflowed(p.src, node, p.eol)))
}
