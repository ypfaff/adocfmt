package printer

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/block"
	"github.com/ypfaff/adocfmt/internal/tree"
)

// headingLine builds the canonical form of a section title: one = per level,
// one space, then the title.
//
// Whether a marker at the end of the line counts is decided against the marker
// in front of it, so changing the marker can turn title text into syntax: the
// title of `## Title ==` is `Title ==`, while `== Title ==` holds `Title`. The
// built line is therefore read back, and one that no longer carries the title
// leaves the node as it stands.
func headingLine(level int, title []byte) ([]byte, bool) {
	line := append(bytes.Repeat([]byte("="), level+1), ' ')
	line = append(line, title...)

	got, ok := block.HeadingTitle(line)
	if !ok || !bytes.Equal(got, title) {
		return nil, false
	}
	return line, true
}

// title emits a section title in its canonical one-line form, whether the
// source wrote it on one line or two, and leaves the node as it stands where
// that form would say something else.
func (p *printer) title(node tree.Node, level int, title tree.Span) {
	if p.raw || node.Base().Frozen {
		p.span(node.Base().Lines)
		return
	}
	line, ok := headingLine(level, p.src[title.Start:title.End])
	if !ok {
		p.span(node.Base().Lines)
		return
	}
	p.out.Write(line)
	lines := node.Base().Lines
	p.out.Write(block.Terminator(p.src[lines.Start:lines.End]))
}

// header rewrites the document title and keeps the author, revision and
// attribute lines below it as they are. A two-line title stays whole, because
// it turns on compat-mode, which changes how inline markup renders.
func (p *printer) header(node *tree.Header) {
	if p.raw || node.Frozen || node.TwoLine {
		p.span(node.Lines)
		return
	}
	line, ok := headingLine(0, p.src[node.Title.Start:node.Title.End])
	if !ok {
		p.span(node.Lines)
		return
	}
	p.out.Write(line)
	p.out.Write(block.Terminator(p.src[node.TitleLines.Start:node.TitleLines.End]))
	p.span(tree.Span{Start: node.TitleLines.End, End: node.Lines.End})
}
