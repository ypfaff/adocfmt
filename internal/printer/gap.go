package printer

import "github.com/ypfaff/adocfmt/internal/block"

// where names which gap policy holds, since the two sides of a gap decide it
// together with what encloses them.
type where int

const (
	betweenSiblings where = iota
	betweenItems
	insideItem
)

// gap emits the blank lines in front of a node.
//
// Asciidoctor renders every block type the same with and without a blank line
// in front of it, so one between siblings costs nothing. Inside a list item it
// would cost: adjacency is what binds a block to the item, and a blank line can
// push a later block out of it, so the rule only collapses a run there.
func (p *printer) gap(g block.Gap, prev block.Node, w where) {
	if p.raw || g.Frozen || keepsGap(prev) {
		p.span(g.Span)
		return
	}
	if prev == nil {
		// A document and a container open on their first node.
		return
	}
	switch {
	case w == insideItem || isContinuation(prev):
		p.collapse(g)
	case w == betweenItems:
		if endsInLiteral(prev) {
			p.blank()
		}
	default:
		p.blank()
	}
}

// tail emits the blank lines at the end of a document or a container, where no
// node follows that could own them.
func (p *printer) tail(g block.Gap) {
	if p.raw || g.Frozen {
		p.span(g.Span)
	}
}

// bound emits the gap between a metadata line and the line or block it binds
// to. The binding survives blank lines, so dropping them changes nothing, with
// two exceptions. A comment binds to nothing at all, because Asciidoctor drops
// it before it renders, and pulling a freestanding comment down onto the block
// below is a change the reader sees while the rendering does not. An attribute
// entry marked open needs the blank line to end its value, see block.Meta.Open.
func (p *printer) bound(g block.Gap, above block.Meta) {
	if p.raw || g.Frozen {
		p.span(g.Span)
		return
	}
	if above.Open || above.Kind == block.MetaComment || above.Kind == block.MetaCommentBlock {
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

func (p *printer) collapse(g block.Gap) {
	if !g.Span.Empty() {
		p.blank()
	}
}

func (p *printer) blank() { p.out.WriteString(p.eol) }

// keepsGap reports whether the gap below a node has to stay as it stands.
// Asciidoctor does not skip front matter by default: it reads the opening ---
// as a thematic break and everything under it as one paragraph, which a blank
// line would end, turning the document title below it into a section.
func keepsGap(prev block.Node) bool {
	_, ok := prev.(*block.FrontMatter)
	return ok
}

// isContinuation reports whether a lone + stands above the gap. Outside a list
// Asciidoctor reads the + and the line under it as one paragraph, and a blank
// line would make two paragraphs of them.
func isContinuation(prev block.Node) bool {
	_, ok := prev.(*block.Continuation)
	return ok
}

// endsInLiteral reports whether the last block of an item is a literal, looked
// up through the lists nested in it. A literal runs to the next blank line, so
// without one it would swallow the marker of the item below.
func endsInLiteral(prev block.Node) bool {
	item, ok := prev.(*block.ListItem)
	if !ok || len(item.Children) == 0 {
		return false
	}
	switch last := item.Children[len(item.Children)-1].(type) {
	case *block.Literal:
		return true
	case *block.List:
		return len(last.Items) > 0 && endsInLiteral(last.Items[len(last.Items)-1])
	default:
		return false
	}
}
