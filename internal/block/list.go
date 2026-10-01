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
		next, ok := s.marker()
		if !ok || next.marker != list.Marker || s.closes(closer) {
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
// asks past the end of the list because nothing closes one: the items the
// directive brings in may still be items of it.
func (s *scanner) directiveBelow() bool {
	m := s.mark()
	defer s.rewind(m)
	s.gap()
	return !s.done() && s.shape().kind == shapeDirective
}

func (s *scanner) listItem(gap Gap, sh shape, closer []byte) *ListItem {
	start := s.pos()
	b := base{gap: gap}
	item := &ListItem{base: b, Marker: sh.span}
	marker := s.at
	bare := sh.span.End == s.lines[marker].text.End

	s.at++
	for !s.done() && !s.closes(closer) {
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
	if bare && (s.at == marker+1 || item.frozen) {
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
	if s.done() || s.closes(closer) || !foldsOntoTerm(s.shape().kind) {
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

// attached reads what hangs off an item: a continuation and the block it
// carries, a nested list, an indented literal block, or whatever follows the
// item's content with no blank line between, which Asciidoctor keeps in the
// item.
func (s *scanner) attached(closer []byte) []Node {
	var children []Node
	for {
		m := s.mark()
		gap := s.gap()
		if s.done() || s.closes(closer) {
			s.rewind(m)
			return children
		}

		sh := s.shape()
		switch {
		case sh.kind == shapeContinuation:
			children = append(children, s.node(gap, closer))
			children = append(children, s.carried(closer)...)
		case sh.kind == shapeMarker && !s.open(sh.marker):
			children = append(children, s.list(base{gap: gap}, sh, closer))
		case bracketed(sh.kind) && s.endsTerm():
			s.rewind(m)
			return children
		case sh.kind == shapeIndented && !gap.Span.Empty():
			children = append(children, s.literal(base{gap: gap}, closer))
		case gap.Span.Empty() && sh.kind != shapeMarker && sh.kind != shapeDelimiter:
			children = append(children, s.adjacent(gap, closer))
		default:
			s.rewind(m)
			return children
		}
	}
}

// endsTerm reports whether the block attribute line the scanner stands on ends
// a description list. It does, unless the next item belongs to a list nested
// in the description list.
//
// This holds even when the line stands in a list nested in the description
// list, because Asciidoctor collects all lines of a term's item, nested lists
// included, before it reads them.
func (s *scanner) endsTerm() bool {
	ending, ok := s.markersEndingTerm()
	if !ok {
		return false
	}
	m := s.mark()
	defer s.rewind(m)
	for !s.done() && (bracketed(s.shape().kind) || s.shape().kind == shapeBlank) {
		s.at++
	}
	next, ok := s.marker()
	return !ok || slices.Contains(ending, next.marker)
}

// markersEndingTerm returns the markers whose items end the innermost open
// description list: its own and those of the lists around it. It reports false
// when no description list is open.
func (s *scanner) markersEndingTerm() ([]string, bool) {
	for i, marker := range slices.Backward(s.markers) {
		if isTermMarker(marker) {
			return s.markers[:i+1], true
		}
	}
	return nil, false
}

// bracketed reports whether the line is one Asciidoctor reads as a block
// attribute line, which is the line that ends a description list. A block title
// is not one of them.
func bracketed(kind shapeKind) bool {
	return kind == shapeAttributes || kind == shapeAnchor
}

// adjacent reads the line that follows an item's content with no blank line
// between. Metadata there binds to a block inside the item, unless the item
// ends first. A delimiter ends it, since a delimited block enters an item only
// behind a continuation. A blank line ends it too, unless a nested list or a
// literal block follows.
func (s *scanner) adjacent(gap Gap, closer []byte) Node {
	b := base{gap: gap}
	for s.metaLine(&b, closer) {
		m := s.mark()
		b.gap = s.gap()
		if !b.gap.Span.Empty() && !s.nests() {
			// The blank lines belong to whatever comes after the item.
			s.rewind(m)
			b.gap = Gap{Span: Span{s.pos(), s.pos()}}
			return s.orphan(b)
		}
	}
	if !s.done() && s.shape().kind == shapeDelimiter {
		return s.orphan(b)
	}
	return s.block(b, closer)
}

// nests reports whether the current line is a nested list item or a literal
// line, the two blocks a blank line does not cut off from the item above.
func (s *scanner) nests() bool {
	if s.done() {
		return false
	}
	sh := s.shape()
	return (sh.kind == shapeMarker && !s.open(sh.marker)) || sh.kind == shapeIndented
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
		if s.done() || s.closes(closer) || s.sibling() {
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

func (s *scanner) marker() (shape, bool) {
	if s.done() {
		return shape{}, false
	}
	sh := s.shape()
	return sh, sh.kind == shapeMarker
}

// open reports whether the marker belongs to a list already being read, in
// which case that list continues rather than a new one nesting.
func (s *scanner) open(marker string) bool {
	return slices.Contains(s.markers, marker)
}

func (s *scanner) inList() bool { return len(s.markers) > 0 }

// sibling reports whether the current line is an item of an open list. It ends
// the item being read whatever came before it, a continuation included.
func (s *scanner) sibling() bool {
	sh, ok := s.marker()
	return ok && s.open(sh.marker)
}
