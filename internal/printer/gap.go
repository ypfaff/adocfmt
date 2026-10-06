package printer

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/block"
)

// where names which gap policy holds, since the two sides of a gap decide it
// together with what encloses them.
type where int

const (
	betweenSiblings where = iota
	betweenItems
	betweenTerms
	insideItem
)

// gap emits the blank lines in front of a node.
//
// Asciidoctor renders every block type the same with and without a blank line
// in front of it, so one between siblings costs nothing. Neither does one
// between two description list terms, where it makes a term easy to find,
// since its marker trails it instead of leading the line. Inside a list item a
// blank line would cost: adjacency is what binds a block to the item, and a
// blank line can push a later block out of it, so the rule only collapses a run
// there.
func (p *printer) gap(g block.Gap, prev block.Node, w where) {
	if p.raw || g.Frozen || isContinuation(prev) {
		p.span(g.Span)
		return
	}
	if prev == nil {
		// A document and a container open on their first node.
		return
	}
	switch w {
	case insideItem:
		p.collapse(g)
	case betweenItems:
		if endsInLiteral(prev) {
			p.collapse(g)
		}
	case betweenTerms:
		if !bareTerm(p.src, prev) {
			p.blank()
		}
	default:
		p.blank()
	}
}

// tail emits the blank lines at the end of a document or a container, where no
// node follows that could own them. Nothing there can merge with or split from
// what precedes it, so even a frozen gap has nothing left to protect and the
// file keeps ending on a single line ending.
func (p *printer) tail(g block.Gap) {
	if p.raw {
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

// isContinuation reports whether a lone + stands above the gap. In a list the
// size of that run is the decision itself: at most one blank line attaches the
// block below to the item, two or more end the list. Outside a list the + is
// prose, and the parser freezes what follows it instead.
func isContinuation(prev block.Node) bool {
	_, ok := prev.(*block.Continuation)
	return ok
}

// endsInLiteral reports whether the last block of an item is a literal, looked
// up through the lists nested in it. A literal runs to the next blank line, so
// without one it would swallow the marker of the item below. Without a blank
// line in the source, something else ended the literal, and adding one could
// end an item around the list instead.
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

// bareTerm reports whether the item is a term with no text and no blocks of its
// own. Asciidoctor gives such a term the description of the term below.
func bareTerm(src []byte, prev block.Node) bool {
	item, ok := prev.(*block.ListItem)
	if !ok || len(item.Children) > 0 {
		return false
	}
	return len(bytes.TrimSpace(src[item.Marker.End:item.Principal.End])) == 0
}
