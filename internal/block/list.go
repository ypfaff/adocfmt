package block

import "slices"

// list reads a run of items sharing one marker. AsciiDoc nests on any change of
// marker, not on indentation, so a different marker opens a nested list and a
// marker of an enclosing list ends this one.
func (p *parser) list(b base, sh shape, closer []byte) Node {
	start, from := p.pos(), p.at
	list := &List{
		base:    b,
		Marker:  sh.marker,
		Open:    slices.Clone(p.markers),
		Section: closer == nil && len(p.markers) == 0,
	}
	p.markers = append(p.markers, sh.marker)
	carrying := p.carrying
	p.carrying = false
	defer func() { p.markers, p.carrying = p.markers[:len(p.markers)-1], carrying }()

	gap := Gap{Span: Span{start, start}}
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
	list.lines = Span{start, p.pos()}
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
func (p *parser) listItem(gap Gap, sh shape, closer []byte) *ListItem {
	start := p.pos()
	item := &ListItem{base: newBase(gap), Marker: sh.span}
	marker := p.at
	bare := sh.span.End == p.lines[marker].text.End

	lines, literals := p.itemEnd(sh, bare, closer)
	p.notes.addLiterals(item, literals)
	p.items = append(p.items, lines)
	defer func() { p.items = p.items[:len(p.items)-1] }()

	p.at++
	for !p.done() {
		inner := p.shape()
		if endsItemText(inner) {
			break
		}
		if inner.kind == shapeDirective {
			p.track(inner)
			item.frozen = true
			p.freeze = true
		}
		p.at++
	}
	item.Principal = Span{start, p.pos()}
	if bare && (p.at == marker+1 || p.directiveSince(start)) {
		p.fold(item, start, closer)
	}

	item.Children = p.attached(closer)
	item.lines = Span{start, p.pos()}
	return item
}

// fold takes the lines under a blank line onto a term that has nothing on its
// line and no line right under it, not even a comment. Asciidoctor reads them
// as the term's text whatever they look like, so a break or a heading among
// them is text, not a block of its own. A directive under the term leaves open
// whether the term has text, so the lines fold into the frozen item then and
// stay as they stand.
func (p *parser) fold(item *ListItem, start int, closer []byte) {
	m := p.mark()
	p.gap()
	if p.done() || !foldsOntoTerm(p.shape().kind) {
		p.rewind(m)
		return
	}

	var folded base
	p.textRun(&folded, closer, endsItemText)
	item.frozen = item.frozen || folded.frozen
	item.Principal = Span{start, p.pos()}
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
func (p *parser) attached(closer []byte) []Node {
	var children []Node
	for {
		m := p.mark()
		gap := p.gap()
		if p.done() {
			p.rewind(m)
			return children
		}
		node := p.node(gap, closer)
		children = append(children, node)
		if _, ok := node.(*Continuation); ok {
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
func (p *parser) carried(closer []byte) []Node {
	p.carrying = true
	defer func() { p.carrying = false }()
	var nodes []Node
	for {
		m := p.mark()
		gap := p.gap()
		if p.done() {
			p.rewind(m)
			return nodes
		}
		node := p.node(gap, closer)
		nodes = append(nodes, node)
		if _, ok := node.(*Attribute); !ok {
			return nodes
		}
	}
}

// siblingAt returns the item on the line at where it continues the list with
// the marker. Asciidoctor takes any line with the separator of a description
// list as its next term, see termShape.
func (p *parser) siblingAt(at int, marker string) (shape, bool) {
	sh := classify(p.src, p.lines[at])
	if isTermMarker(marker) {
		sh, _ = termShape(p.src, p.lines[at])
	}
	return sh, sh.kind == shapeMarker && sh.marker == marker
}

func (p *parser) inList() bool { return len(p.markers) > 0 }
