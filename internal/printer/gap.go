package printer

import "github.com/ypfaff/adocfmt/internal/tree"

// where names which gap policy holds, since the two sides of a gap decide it
// together with what encloses them.
type where int

const (
	betweenSiblings where = iota
	betweenItems
	betweenTerms
)

// gap emits the blank lines in front of a node, as far as its stage leaves
// them to the rule: a gap that keeps its count collapses to one blank line, as
// does every gap inside a list item, see block.attached.
//
// Asciidoctor renders every block type the same with and without a blank line
// in front of it, so one between siblings costs nothing. Neither does one
// between two description list terms, where it shows where an entry with
// blocks of its own ends. The other items the rule writes without one.
func (p *printer) gap(g tree.Gap, prev tree.Node, w where) {
	if p.raw || g.Stage == tree.Exact {
		p.span(g.Span)
		return
	}
	if prev == nil {
		// A document and a container open on their first node.
		return
	}
	if g.Stage == tree.KeepCount {
		p.collapse(g)
		return
	}
	switch w {
	case betweenSiblings:
		p.blank()
	case betweenTerms:
		if hasBlocks(prev) {
			p.blank()
		}
	}
}

// tail emits the blank lines at the end of a document or a container, where no
// node follows that could own them. Nothing there can merge with or split from
// what precedes it, so the parser leaves the gap free, and the file keeps
// ending on a single line ending.
func (p *printer) tail(g tree.Gap) {
	if p.raw {
		p.span(g.Span)
	}
}

// bound emits the gap between a metadata line and the line or block it binds
// to. The binding survives blank lines, so dropping them changes nothing,
// unless the gap keeps its count, or a comment stands above it. A comment
// binds to nothing at all, because Asciidoctor drops it before it renders, and
// pulling a freestanding comment down onto the block below is a change the
// reader sees while the rendering does not.
func (p *printer) bound(g tree.Gap, above tree.Meta) {
	if p.raw || g.Stage == tree.Exact {
		p.span(g.Span)
		return
	}
	if g.Stage == tree.KeepCount || above.Kind == tree.MetaComment || above.Kind == tree.MetaCommentBlock {
		p.collapse(g)
	}
}

// finish ends the last line of the document, which the source may leave open.
func (p *printer) finish() {
	out := p.out.Bytes()
	if p.raw || len(out) == p.start || out[len(out)-1] == '\n' {
		return
	}
	p.out.WriteString(p.eol)
}

func (p *printer) collapse(g tree.Gap) {
	if !g.Span.Empty() {
		p.blank()
	}
}

func (p *printer) blank() { p.out.WriteString(p.eol) }

// hasBlocks reports whether the item holds blocks besides its text, such as a
// paragraph attached with a + or a nested list.
func hasBlocks(node tree.Node) bool {
	item, ok := node.(*tree.ListItem)
	return ok && len(item.Children) > 0
}
