package block

import (
	"slices"
	"strings"
)

// list reads a run of items sharing one marker. AsciiDoc nests on any change of
// marker, not on indentation, so a different marker opens a nested list and a
// marker of an enclosing list ends this one.
func (s *scanner) list(b base, sh shape, closer []byte) Node {
	start := s.pos()
	list := &List{base: b, Marker: sh.marker}
	s.markers = append(s.markers, sh.marker)
	defer func() { s.markers = s.markers[:len(s.markers)-1] }()

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
	bare := isTerm(sh.marker) && sh.span.End == s.lines[s.at].text.End

	s.at++
	for !s.done() && !s.closes(closer) {
		inner := s.shape()
		if inner.kind == shapeBlank || inner.kind == shapeMarker || inner.kind == shapeDelimiter ||
			inner.kind == shapeAttributes || inner.kind == shapeContinuation {
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
	s.textRun(&folded, closer)
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

func isTerm(marker string) bool {
	return strings.HasPrefix(marker, "::") || marker == ";;"
}

// attached reads what hangs off an item: a continuation and the block it
// carries, a nested list, or an indented literal block.
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
		default:
			s.at, s.freeze = at, freeze
			return children
		}
	}
}

// carried is the block a continuation line attaches to the item. A continuation
// with nothing left to carry gives the blank lines back, so they end up in the
// tail of the enclosing block rather than in no node at all.
func (s *scanner) carried(closer []byte) Node {
	at, freeze := s.at, s.freeze
	gap := s.gap()
	if s.done() || s.closes(closer) {
		s.at, s.freeze = at, freeze
		return nil
	}
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
