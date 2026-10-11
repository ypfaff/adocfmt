package block

import (
	"slices"

	"github.com/ypfaff/adocfmt/internal/tree"
)

// list reads a run of items sharing one marker. AsciiDoc nests on any change of
// marker, not on indentation, so a different marker opens a nested list and a
// marker of an enclosing list ends this one.
func (p *parser) list(b tree.NodeBase, sh shape, closer []byte) tree.Node {
	start, from := p.pos(), p.at
	list := &tree.List{
		NodeBase:    b,
		Marker:      sh.marker,
		Open:        slices.Clone(p.markers),
		Section:     closer == nil && len(p.markers) == 0,
		Description: isTermMarker(sh.marker),
	}
	p.markers = append(p.markers, sh.marker)
	carrying := p.carrying
	p.carrying = false
	defer func() { p.markers, p.carrying = p.markers[:len(p.markers)-1], carrying }()

	gap := tree.Gap{Span: tree.Span{Start: start, End: start}}
	for {
		list.Items = append(list.Items, p.listItem(gap, sh, closer))

		m := p.mark()
		gap = p.gap()
		if p.done() {
			p.rewind(m)
			break
		}
		next, ok := p.siblingAt(p.at, list.Marker)
		if !ok {
			p.rewind(m)
			break
		}
		sh = next
	}
	list.Lines = tree.Span{Start: start, End: p.pos()}
	list.Extensible = p.extensible(false, from) || p.directiveBelow()
	return list
}

// directiveBelow reports whether a directive stands right under the list. It
// asks past the end of the list, and past the end of the item it stands in,
// because nothing closes one: the items the directive brings in may still be
// items of it.
func (p *parser) directiveBelow() bool {
	m, items := p.mark(), p.items
	defer func() { p.rewind(m); p.items = items }()
	p.items = nil
	p.gap()
	return !p.done() && p.shape().kind == shapeDirective
}

// listItem reads the item on the current line in two steps, as Asciidoctor
// does: itemEnd finds the lines the item takes, and the parser reads the
// item's text and blocks from those lines only.
func (p *parser) listItem(gap tree.Gap, sh shape, closer []byte) *tree.ListItem {
	start := p.pos()
	item := &tree.ListItem{NodeBase: newBase(gap), Marker: sh.span}
	marker := p.at
	bare := sh.span.End == p.lines[marker].text.End

	lines, literals := p.itemEnd(sh, bare, closer)
	p.notes.addLiterals(item, literals)
	p.items = append(p.items, lines)
	defer func() { p.items = p.items[:len(p.items)-1] }()

	p.at++
	for !p.done() {
		inner := p.shape()
		if p.endsItemText(inner) {
			break
		}
		if inner.kind == shapeDirective {
			p.track(inner)
			item.Frozen = true
			p.freeze = true
		}
		p.at++
	}
	item.Principal = tree.Span{Start: start, End: p.pos()}
	if bare && (p.at == marker+1 || p.directiveSince(start)) {
		p.fold(item, start, closer)
	}
	item.Text = p.itemText(marker, sh.span.End)

	// The text under the item's line is its first block. Comment lines alone
	// are not: Asciidoctor reads them as the metadata of the block below.
	if p.onlyComments(marker + 1) {
		p.first = firstBlock{textOnly: p.startsTextOnly(marker, sh, bare), opened: p.at > marker+1}
	}
	item.Children = p.attached(closer)
	p.first = firstBlock{}
	item.Lines = tree.Span{Start: start, End: p.pos()}
	return item
}

// startsTextOnly reports whether Asciidoctor reads the first block of the item
// on the line marker as text only: a term without text does, and an item of
// another list where the line under it, past comment lines, is neither blank
// nor a +, which the item's buffer holds as a blank line.
// Ruby: text_only: has_text ? nil : true, in parse_list_item.
func (p *parser) startsTextOnly(marker int, sh shape, bare bool) bool {
	if isTermMarker(sh.marker) {
		return bare
	}
	at := marker + 1
	for at < p.limit() && classify(p.src, p.lines[at]).kind == shapeComment {
		at++
	}
	if at == p.limit() {
		return false
	}
	kind := classify(p.src, p.lines[at]).kind
	return kind != shapeBlank && kind != shapeContinuation
}

// onlyComments reports whether the lines from the line at up to the current
// one are all comment lines.
func (p *parser) onlyComments(at int) bool {
	for ; at < p.at; at++ {
		if classify(p.src, p.lines[at]).kind != shapeComment {
			return false
		}
	}
	return true
}

// itemText finds the text of the item whose marker line is marker and which
// has read its text up to the current line. The text starts behind the spaces
// after the marker, or, where nothing stands there, on the first line under it
// that holds any.
func (p *parser) itemText(marker, behind int) tree.Span {
	text := p.lines[marker].text
	for behind < text.End && (p.src[behind] == ' ' || p.src[behind] == '\t') {
		behind++
	}
	span := tree.Span{Start: behind, End: text.End}
	for at := marker + 1; at < p.at; at++ {
		line := p.lines[at].text
		if line.Empty() {
			continue
		}
		if span.Empty() {
			span.Start = line.Start
		}
		span.End = line.End
	}
	return span
}

// fold takes the lines under a blank line onto a term that has nothing on its
// line and no line right under it, not even a comment. Asciidoctor reads them
// as the term's text whatever they look like, so a break or a heading among
// them is text, not a block of its own. Only a line that reads as an item is
// not, see endsItemText. A directive under the term leaves open whether the
// term has text, so the lines fold into the frozen item then and stay as they
// stand.
func (p *parser) fold(item *tree.ListItem, start int, closer []byte) {
	m := p.mark()
	p.gap()
	if p.done() || !foldsOntoTerm(p.shape().kind) || p.endsItemText(p.shape()) {
		p.rewind(m)
		return
	}

	var folded tree.NodeBase
	p.textRun(&folded, closer, p.endsItemText)
	item.Frozen = item.Frozen || folded.Frozen
	item.Principal = tree.Span{Start: start, End: p.pos()}
	// Folding keeps the gap in Principal instead of giving it back, so the
	// freeze that reading it consumed has to carry past the folded lines.
	p.freeze = p.freeze || m.freeze
}

func foldsOntoTerm(kind shapeKind) bool {
	switch kind {
	case shapeText, shapeHeading, shapeMacro, shapeQuote, shapeBreak:
		return true
	default:
		return false
	}
}

// attached reads the blocks of an item under its text, down to the end
// itemEnd found for it. Asciidoctor reads them from the lines it collected for
// the item, so whatever stands there belongs to the item, even a line that
// reads as an item of an open list: that one opens a nested list.
func (p *parser) attached(closer []byte) []tree.Node {
	var children []tree.Node
	for {
		m := p.mark()
		gap := p.gap()
		if p.done() {
			p.rewind(m)
			return children
		}
		node := p.node(gap, closer)
		children = append(children, node)
		if _, ok := node.(*tree.Continuation); ok {
			children = append(children, p.carried(closer)...)
		}
	}
}

// carried reads the blocks a continuation line attaches to the item. That is
// the block under it. Where attribute entries come first, it is those entries
// and the block under them, because Asciidoctor applies an entry and attaches
// the next block instead.
//
// A continuation with nothing left to carry gives the blank lines back, so they
// end up in the gap of the next item or the tail of the enclosing block rather
// than in no node at all.
func (p *parser) carried(closer []byte) []tree.Node {
	p.carrying = true
	defer func() { p.carrying = false }()
	var nodes []tree.Node
	for {
		m := p.mark()
		gap := p.gap()
		if p.done() {
			p.rewind(m)
			return nodes
		}
		node := p.node(gap, closer)
		nodes = append(nodes, node)
		if _, ok := node.(*tree.Attribute); !ok {
			return nodes
		}
	}
}

// siblingAt returns the item on the line at where it continues the list with
// the marker. Asciidoctor reads the line as an item of that list whatever else
// it reads as, see listShape and termShape.
func (p *parser) siblingAt(at int, marker string) (shape, bool) {
	l := p.lines[at]
	sh, ok := listShape(p.text(l), l.text.Start)
	if isTermMarker(marker) {
		sh, ok = termShape(p.src, l)
	}
	return sh, ok && sh.marker == marker
}

func (p *parser) inList() bool { return len(p.markers) > 0 }
