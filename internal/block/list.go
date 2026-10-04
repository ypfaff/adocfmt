package block

import "slices"

// list reads a run of items sharing one marker. AsciiDoc nests on any change of
// marker, not on indentation, so a different marker opens a nested list and a
// marker of an enclosing list ends this one.
func (s *scanner) list(b base, sh shape, closer []byte) Node {
	start, from := s.pos(), s.at
	list := &List{
		base:    b,
		Marker:  sh.marker,
		Open:    slices.Clone(s.markers),
		Section: closer == nil && len(s.markers) == 0,
	}
	s.markers = append(s.markers, sh.marker)
	carrying := s.carrying
	s.carrying = false
	defer func() { s.markers, s.carrying = s.markers[:len(s.markers)-1], carrying }()

	gap := Gap{Span: Span{start, start}}
	for {
		list.Items = append(list.Items, s.listItem(gap, sh, closer))

		m := s.mark()
		gap = s.gap()
		if s.done() {
			s.rewind(m)
			break
		}
		next, ok := s.siblingAt(s.at, list.Marker)
		if !ok {
			s.rewind(m)
			break
		}
		sh = next
	}
	list.lines = Span{start, s.pos()}
	list.Extensible = s.extensible(false, from) || s.directiveBelow()
	return list
}

// directiveBelow reports whether a directive stands right under the list. It
// asks past the end of the list, and past the end of the item it stands in,
// because nothing closes one: the items the directive brings in may still be
// items of it.
func (s *scanner) directiveBelow() bool {
	m, items := s.mark(), s.items
	defer func() { s.rewind(m); s.items = items }()
	s.items = nil
	s.gap()
	return !s.done() && s.shape().kind == shapeDirective
}

// listItem reads the item on the current line in two steps, as Asciidoctor
// does: itemEnd finds the lines the item takes, and the scanner reads the
// item's text and blocks from those lines only.
func (s *scanner) listItem(gap Gap, sh shape, closer []byte) *ListItem {
	start := s.pos()
	item := &ListItem{base: newBase(gap), Marker: sh.span}
	marker := s.at
	bare := sh.span.End == s.lines[marker].text.End

	lines, literals := s.itemEnd(sh, bare, closer)
	s.notes.addLiterals(item, literals)
	s.items = append(s.items, lines)
	defer func() { s.items = s.items[:len(s.items)-1] }()

	s.at++
	for !s.done() {
		inner := s.shape()
		if endsItemText(inner) {
			break
		}
		if inner.kind == shapeDirective {
			s.track(inner)
			item.frozen = true
			s.freeze = true
		}
		s.at++
	}
	item.Principal = Span{start, s.pos()}
	if bare && (s.at == marker+1 || s.directiveSince(start)) {
		s.fold(item, start, closer)
	}

	item.Children = s.attached(closer)
	item.lines = Span{start, s.pos()}
	return item
}

// fold takes the lines under a blank line onto a term that has nothing on its
// line and no line right under it, not even a comment. Asciidoctor reads them
// as the term's text whatever they look like, so a break or a heading among
// them is text, not a block of its own. A directive under the term leaves open
// whether the term has text, so the lines fold into the frozen item then and
// stay as they stand.
func (s *scanner) fold(item *ListItem, start int, closer []byte) {
	m := s.mark()
	s.gap()
	if s.done() || !foldsOntoTerm(s.shape().kind) {
		s.rewind(m)
		return
	}

	var folded base
	s.textRun(&folded, closer, endsItemText)
	item.frozen = item.frozen || folded.frozen
	item.Principal = Span{start, s.pos()}
	// Folding keeps the gap in Principal instead of giving it back, so the
	// freeze that reading it consumed has to carry past the folded lines.
	s.freeze = s.freeze || m.freeze
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
func (s *scanner) attached(closer []byte) []Node {
	var children []Node
	for {
		m := s.mark()
		gap := s.gap()
		if s.done() {
			s.rewind(m)
			return children
		}
		node := s.node(gap, closer)
		children = append(children, node)
		if _, ok := node.(*Continuation); ok {
			children = append(children, s.carried(closer)...)
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
func (s *scanner) carried(closer []byte) []Node {
	s.carrying = true
	defer func() { s.carrying = false }()
	var nodes []Node
	for {
		m := s.mark()
		gap := s.gap()
		if s.done() {
			s.rewind(m)
			return nodes
		}
		node := s.node(gap, closer)
		nodes = append(nodes, node)
		if _, ok := node.(*Attribute); !ok {
			return nodes
		}
	}
}

// siblingAt returns the item on the line at where it continues the list with
// the marker. Asciidoctor takes any line with the separator of a description
// list as its next term, see termShape.
func (s *scanner) siblingAt(at int, marker string) (shape, bool) {
	sh := classify(s.src, s.lines[at])
	if isTermMarker(marker) {
		sh, _ = termShape(s.src, s.lines[at])
	}
	return sh, sh.kind == shapeMarker && sh.marker == marker
}

func (s *scanner) inList() bool { return len(s.markers) > 0 }
