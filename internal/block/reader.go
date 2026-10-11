package block

import "bytes"

// reader holds the lines the parser may read for one block: from where it
// stands up to end.
//
// Asciidoctor first collects the lines of a list item, then reads the item's
// blocks from those lines alone, so nothing it reads runs past the item. The
// parser does the same: listItem enters a reader that ends where the item
// ends, and leaves it once the item is read. A nested item enters its own
// reader inside that one, so the readers form a chain, with the innermost in
// parser.reader.
//
// A delimited block enters a reader too, which ends at its closing line, see
// closer.
type reader struct {
	// outer is the reader around this one, nil for the whole source.
	outer *reader
	// end is the first line the reader no longer holds.
	end int
	// closer is the closing line of a delimited block, nil for an item and the
	// source. A delimited block does not know in advance where it ends, so its
	// reader stops at the first line equal to closer. An item does: itemEnd works
	// out its last line before the item is read, and end holds it.
	closer []byte
	// item is how Asciidoctor's buffer for the item holds its lines: which it
	// leaves out, and which + it reads as a blank line. nil for the source and
	// a delimited block.
	item *itemLines
}

// enter makes r the reader the parser reads from, until leave.
func (p *parser) enter(r *reader) {
	r.outer = p.reader
	p.reader = r
}

// enterBlock makes the parser read the delimited block that closer closes.
// A delimiter closes only on a line equal to the one that opened it.
func (p *parser) enterBlock(closer []byte) {
	p.enter(&reader{end: p.reader.end, closer: closer})
}

func (p *parser) leave() { p.reader = p.reader.outer }

func (p *parser) done() bool { return p.past(p.at) }

// past reports whether the line at is past the lines the reader holds: at its
// end or on its closing line. Only the reader's own closer counts, as only the
// innermost block is open for Asciidoctor, see closesInside.
func (p *parser) past(at int) bool {
	r := p.reader
	return at >= r.end || r.closer != nil && bytes.Equal(p.text(p.lines[at]), r.closer)
}

// atSectionLevel reports whether the parser reads outside every delimited
// block and list item, the only place where sections exist.
func (p *parser) atSectionLevel() bool { return p.reader.outer == nil }

// skipped and blanked report whether the buffer of an item being read leaves
// the line out, or holds the + on it as a blank line. Every one of them counts:
// Asciidoctor reads a nested item from the buffer of its parent, which it read
// from the grandparent's.
func (r *reader) skipped(at int) bool {
	return r.anyItem(func(l *itemLines) bool { return l.skipped[at] })
}

func (r *reader) blanked(at int) bool {
	return r.anyItem(func(l *itemLines) bool { return l.blanked == at })
}

// placeholder reports whether the buffer of an item being read holds the + on
// the line as a blank line for the blocks read from it. A nested item reads
// one that stands under a line as a + still, so blanked leaves it out.
func (r *reader) placeholder(at int) bool {
	return r.anyItem(func(l *itemLines) bool { return l.placeholders[at] })
}

// anyItem reports whether holds is true for the buffer of any item the reader
// reads from.
func (r *reader) anyItem(holds func(*itemLines) bool) bool {
	for ; r != nil; r = r.outer {
		if r.item != nil && holds(r.item) {
			return true
		}
	}
	return false
}
