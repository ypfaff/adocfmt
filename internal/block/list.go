package block

import "slices"

// list reads a run of items sharing one marker. AsciiDoc nests on any change of
// marker, not on indentation, so a different marker opens a nested list and a
// marker of an enclosing list ends this one.
func (s *scanner) list(b base, sh shape, closer []byte) Node {
	start := s.pos()
	list := &List{base: b, Marker: sh.marker}
	s.markers = append(s.markers, sh.marker)
	carrying := s.carrying
	s.carrying = false
	defer func() { s.markers, s.carrying = s.markers[:len(s.markers)-1], carrying }()

	gap := Gap{Span: Span{start, start}}
	for {
		list.Items = append(list.Items, s.listItem(gap, sh, closer))

		at, freeze := s.at, s.freeze
		gap = s.gap()
		next, ok := s.marker()
		if !ok || next.marker != list.Marker || s.closes(closer) {
			s.at, s.freeze = at, freeze
			break
		}
		sh = next
	}
	list.Lines = Span{start, s.pos()}
	return list
}

func (s *scanner) listItem(gap Gap, sh shape, closer []byte) *ListItem {
	start := s.pos()
	b := base{Gap: gap}
	item := &ListItem{base: b, Marker: sh.span}
	bare := sh.span.End == s.lines[s.at].text.End

	s.at++
	for !s.done() && !s.closes(closer) {
		inner := s.shape()
		if endsItemText(inner) {
			break
		}
		if inner.kind == shapeDirective {
			s.track(inner)
			item.Frozen = true
			s.freeze = true
		}
		s.at++
	}
	item.Principal = Span{start, s.pos()}
	if bare {
		s.fold(item, start, closer)
	}

	item.Children = s.attached(closer)
	item.Lines = Span{start, s.pos()}
	return item
}

// fold takes the lines after a term that carries no text of its own onto that
// term. Asciidoctor reads them as the term's text whatever they look like, so
// a break or a heading among them is text, not a block of its own.
func (s *scanner) fold(item *ListItem, start int, closer []byte) {
	at, freeze := s.at, s.freeze
	s.gap()
	if s.done() || s.closes(closer) || !foldsOntoTerm(s.shape().kind) {
		s.at, s.freeze = at, freeze
		return
	}

	var folded base
	s.textRun(&folded, closer, endsItemText)
	item.Frozen = item.Frozen || folded.Frozen
	item.Principal = Span{start, s.pos()}
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
		at, freeze := s.at, s.freeze
		gap := s.gap()
		if s.done() || s.closes(closer) {
			s.at, s.freeze = at, freeze
			return children
		}

		sh := s.shape()
		switch {
		case sh.kind == shapeContinuation:
			children = append(children, s.node(gap, closer))
			if carried := s.carried(closer); carried != nil {
				children = append(children, carried)
			}
		case sh.kind == shapeMarker && !s.open(sh.marker):
			children = append(children, s.list(base{Gap: gap}, sh, closer))
		case sh.kind == shapeIndented && !gap.Span.Empty():
			children = append(children, s.literal(base{Gap: gap}, closer))
		case gap.Span.Empty() && sh.kind != shapeMarker && sh.kind != shapeDelimiter:
			children = append(children, s.adjacent(gap, closer))
		default:
			s.at, s.freeze = at, freeze
			return children
		}
	}
}

// adjacent reads the line that follows an item's content with no blank line
// between. Metadata there binds to a block inside the item, or to none when
// the item ends first: at a delimiter, which enters an item only behind a
// continuation, or at a blank line, unless a nested list or a literal block
// follows it.
func (s *scanner) adjacent(gap Gap, closer []byte) Node {
	b := base{Gap: gap}
	for s.metaLine(&b, closer) {
		at, freeze := s.at, s.freeze
		if b.Gap = s.gap(); b.Gap.Span.Empty() || s.nests() {
			continue
		}
		// The blank lines belong to whatever comes after the item.
		s.at, s.freeze = at, freeze
		b.Gap = Gap{Span: Span{s.pos(), s.pos()}}
		return s.block(b, closer, true)
	}
	return s.block(b, closer, !s.done() && s.shape().kind == shapeDelimiter)
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

// carried is the block a continuation line attaches to the item. A continuation
// with nothing left to carry gives the blank lines back, so they end up in the
// gap of the next item or the tail of the enclosing block rather than in no
// node at all.
func (s *scanner) carried(closer []byte) Node {
	at, freeze := s.at, s.freeze
	gap := s.gap()
	if s.done() || s.closes(closer) || s.sibling() {
		s.at, s.freeze = at, freeze
		return nil
	}
	s.carrying = true
	defer func() { s.carrying = false }()
	return s.node(gap, closer)
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
