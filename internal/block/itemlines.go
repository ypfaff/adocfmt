package block

import (
	"bytes"

	"github.com/ypfaff/adocfmt/internal/tree"
)

// itemLines is how the buffer Asciidoctor collects for a list item holds its
// lines, see itemEnd.
type itemLines struct {
	// first is the line under the item's line, where the buffer starts.
	first int
	// skipped are the blank lines the buffer leaves out, see reader.skipped.
	skipped map[int]bool
	// blanked is the + under a blank line, or -1. A nested item reads it as
	// a blank line too, see reader.blanked.
	blanked int
	// placeholders are the + lines the item's own blocks read as blank lines:
	// blanked, and each + under a line that carries the line below it. A
	// nested item never reads one of the latter as a blank line: it stands
	// above the nested list, or a callout list reads it as a + still.
	placeholders map[int]bool
	// literals are the literal paragraphs the buffer collected whole, see
	// takeLiteral.
	literals []tree.Span
}

// itemEnd finds where the item on the current line ends, and returns the
// reader of its lines. Asciidoctor collects the lines of an item into a buffer
// before it reads any block in them, so a block does not decide where the item
// ends; the item decides where the block may end.
//
// Where it finds blank lines that decide which lines the item takes, it notes
// how far their gap may change in parser.significant, which gap reads.
//
// It ports read_lines_for_list_item from Asciidoctor's parser.rb. A comment
// starting with "Ruby:" quotes the code a part stands for, "Not in parser.rb:"
// marks what the port adds, and "Not ported:" what it leaves out and why.
func (p *parser) itemEnd(sh shape, bare bool) *reader {
	dlist := isTermMarker(sh.marker)
	r := itemReader{
		p: p, marker: sh.marker, dlist: dlist,
		hasText: !dlist || !bare, at: p.at + 1, detached: -1,
		skipped: map[int]bool{}, placeholders: map[int]bool{},
	}
	r.end = r.at
	for r.hasMoreLines() {
		if !r.step() {
			break
		}
	}
	// Ruby: buffer[detached_continuation] = ListContinuationPlaceholder
	if r.detached >= 0 {
		r.placeholders[r.detached] = true
	}
	lines := &itemLines{
		first: p.at + 1, skipped: r.skipped, blanked: r.detached,
		placeholders: r.placeholders, literals: r.literals,
	}
	return &reader{end: r.end, item: lines}
}

// itemReader holds the state of read_lines_for_list_item.
type itemReader struct {
	p *parser
	// marker is sibling_trait.
	marker string
	dlist  bool

	hasText      bool
	continuation continuation
	withinNested bool
	prev         prevLine
	// plus and blank are the lines of the last + and the last blank line added
	// to the buffer.
	plus  int
	blank int
	// detached is the line of the last + under a blank line, or -1.
	detached int

	at  int
	end int
	// skipped, placeholders and literals are what itemLines says they are.
	skipped      map[int]bool
	placeholders map[int]bool
	literals     []tree.Span
}

// continuation is the state of the + lines, with Asciidoctor's names.
type continuation int

const (
	continuationInactive continuation = iota
	continuationActive
	continuationFrozen
)

// prevLine is what prev_line holds: the last line added to the buffer.
type prevLine int

const (
	prevNone prevLine = iota
	prevEmpty
	prevText
	prevPlus
)

// take adds the current line to the buffer: buffer << this_line. The item's
// span covers it too, except blank lines at the end, which Asciidoctor trims
// from the buffer. A + at the end stays in the span although Asciidoctor pops
// it, since the item consumes it.
func (r *itemReader) take() {
	switch r.shape(r.at).kind {
	case shapeBlank:
		r.prev, r.blank = prevEmpty, r.at
	case shapeContinuation:
		r.prev, r.end, r.plus = prevPlus, r.at+1, r.at
	default:
		r.prev, r.end = prevText, r.at+1
	}
	r.at++
}

// drop moves past the current line without adding it to the buffer; the span
// still covers it. Unlike a skipped blank line it is not recorded, since an
// item nested in this one drops it on its own.
func (r *itemReader) drop() {
	r.at++
	r.end = r.at
}

// step reads the current line, one turn of the Ruby loop, and reports whether
// the item goes on below it.
func (r *itemReader) step() bool {
	// Not in parser.rb: a line an enclosing item skipped is not in the buffer
	// this one reads from.
	if r.p.reader.skipped(r.at) {
		r.at++
		return true
	}
	sh := r.shape(r.at)
	// Ruby: break if is_sibling_list_item?
	if r.siblingItem(r.at) {
		return false
	}
	// Not in parser.rb: a directive reads as bringing in nothing, and the
	// blocks read from the buffer freeze around it, see reach.
	if sh.kind == shapeDirective {
		r.drop()
		return true
	}
	// Ruby: if ListContinuationMarker === prev_line
	if r.prev == prevPlus {
		if r.afterContinuation(sh) {
			return true
		}
	}

	switch {
	// Ruby: if (match = is_delimited_block? this_line, true)
	case sh.kind == shapeDelimiter:
		if r.continuation != continuationActive {
			return false
		}
		r.takeBlock(sh)
		r.continuation = continuationInactive
	// Ruby: elsif dlist && continuation != :active && BlockAttributeLineRx
	case r.dlist && r.continuation != continuationActive && bracketed(sh.kind):
		return r.takeAttributes()
	// Ruby: elsif continuation == :active && !this_line.empty?
	case r.continuation == continuationActive && sh.kind != shapeBlank:
		r.takeContinued(sh)
	// Ruby: elsif prev_line && prev_line.empty?
	case r.prev == prevEmpty:
		return r.afterBlank(sh)
	// Ruby: elsif ListContinuationMarker === this_line
	case sh.kind == shapeContinuation:
		r.hasText = true
		r.take()
	// Ruby: else
	default:
		r.noteAwaitedText(r.at)
		r.hasText = r.hasText || sh.kind != shapeBlank
		r.noteTermOrItem(r.at)
		r.noteNestedList(r.nestable())
		r.take()
	}
	return true
}

// afterContinuation reads the line under a +, which activates an inactive
// continuation. It reports whether the line is a + too: the first such + freezes
// the continuation and goes into the buffer, any further one is dropped.
func (r *itemReader) afterContinuation(sh shape) bool {
	if r.continuation == continuationInactive {
		r.continuation, r.hasText = continuationActive, true
		// Ruby: buffer[-1] = ListContinuationPlaceholder unless within_nested_list
		if !r.withinNested {
			r.placeholders[r.plus] = true
		}
	}
	// Ruby: if ListContinuationMarker === this_line
	if sh.kind != shapeContinuation {
		return false
	}
	if r.continuation == continuationFrozen {
		r.drop()
		return true
	}
	r.continuation = continuationFrozen
	r.take()
	return true
}

// takeBlock takes a delimited block whole, down to its closing line.
// Ruby: read_lines_until terminator
func (r *itemReader) takeBlock(sh shape) {
	closer := r.text(r.at)[:sh.width]
	r.take()
	for r.hasMoreLines() {
		closes := bytes.Equal(r.text(r.at), closer)
		r.take()
		if closes {
			return
		}
	}
}

// takeAttributes reads the block attribute lines a description list meets
// outside a +. They stay only where an item of another list follows them,
// blank lines between allowed; anything else ends the item above them. Where
// the lines run out under them, Asciidoctor reads them and renders them
// nowhere; the item's span takes them.
func (r *itemReader) takeAttributes() bool {
	next := r.at + 1
	// Ruby: while (next_line = reader.peek_line)
	for r.within(next) {
		kind := r.shape(next).kind
		// Ruby: elsif next_line.empty? || BlockAttributeLineRx.match? next_line
		if kind != shapeBlank && !bracketed(kind) {
			break
		}
		next++
	}
	if r.within(next) {
		// Ruby: if is_delimited_block? next_line then interrupt
		// Ruby: elsif AnyListRx.match? next_line && !is_sibling_list_item? then keep
		keep := r.shape(next).kind != shapeDelimiter &&
			opensAnyList(r.p.src, r.p.lines[next]) && !r.siblingItem(next)
		if !keep {
			return false
		}
	}
	for r.at < next {
		r.take()
	}
	return true
}

// takeContinued takes a line the active continuation reaches. A literal
// paragraph is collected whole, and metadata lines leave the continuation
// active for the block below them.
func (r *itemReader) takeContinued(sh shape) {
	switch {
	// Ruby: if LiteralParagraphRx.match? this_line
	case isIndented(r.text(r.at)):
		r.takeLiteral()
		r.continuation = continuationInactive
	// Ruby: elsif BlockTitleRx, BlockAttributeLineRx or AttributeEntryRx
	case sh.kind == shapeTitle || sh.kind == shapeAttrEntry || bracketed(sh.kind):
		r.take()
	// Ruby: else
	default:
		r.noteNestedList(r.nestable())
		r.take()
		r.continuation = continuationInactive
	}
}

// afterBlank reads the line under a blank one. There the item goes on only at
// a +, a nested list or a literal paragraph, or where a term without text
// still waits for it.
func (r *itemReader) afterBlank(sh shape) bool {
	// Ruby: if this_line.empty?
	if sh.kind == shapeBlank {
		// Ruby: reader.skip_blank_lines
		blankedPlus := false
		for r.hasMoreLines() && r.shape(r.at).kind == shapeBlank {
			blankedPlus = blankedPlus || r.p.reader.blanked(r.at)
			r.skipped[r.at] = true
			r.at++
		}
		// Ruby: break unless (this_line = ... && reader.read_line)
		if !r.hasMoreLines() {
			return false
		}
		// Ruby: break if is_sibling_list_item? this_line
		if r.siblingItem(r.at) {
			// Not in parser.rb: the run held a + an enclosing item blanked
			// out. Asciidoctor reads past it to the next item, but in the tree
			// the + is a node: the item takes it, so it does not stand between
			// this item and the next.
			if blankedPlus {
				r.end = r.at
			}
			return false
		}
		sh = r.shape(r.at)
		r.noteRun(sh)
	}
	r.noteTermOrItem(r.at)

	switch {
	// Ruby: if this_line == LIST_CONTINUATION
	case sh.kind == shapeContinuation:
		r.detached = r.at
		r.take()
	// Ruby: else, for a term still waiting for text. Moved up, so the cases
	// below are Ruby's elsif has_text branch; its is_sibling_list_item? check
	// already ran in step or after the run.
	case !r.hasText:
		r.noteAwaitedText(r.at)
		// Ruby: buffer.pop unless within_nested_list
		if !r.withinNested {
			r.skipped[r.blank] = true
		}
		r.hasText = true
		r.take()
	// Ruby: elsif (nested_list_type = NESTABLE_LIST_CONTEXTS.find ...)
	case r.noteNestedList(nestableLists):
		r.take()
	// Ruby: elsif LiteralParagraphRx.match? this_line
	case isIndented(r.text(r.at)):
		r.takeLiteral()
	// Ruby: else break
	default:
		return false
	}
	return true
}

// noteRun notes a run of blank lines above the current line that one blank
// line would not replace, so its gap stays exact.
//
// Not in parser.rb. Under one blank line, the next turn of the loop checks a
// delimiter, a dlist attribute line and an active continuation before
// prev_line.empty?; under a run, skip_blank_lines reads the line past those
// checks.
func (r *itemReader) noteRun(sh shape) {
	// A delimiter ends the item either way, unless a term still waits for text.
	delimiter := sh.kind == shapeDelimiter && !r.hasText
	attributes := r.dlist && bracketed(sh.kind)
	if r.continuation == continuationActive || delimiter || attributes {
		r.note(r.at, tree.Exact)
	}
}

// note records the stage of the gap above the line at, see parser.significant.
func (r *itemReader) note(at int, stage tree.Stage) {
	r.p.significant[at] = max(r.p.significant[at], stage)
}

// takeLiteral takes an indented line with the lines under it up to a blank
// line or a +, whatever they look like, since Asciidoctor collects a literal
// paragraph whole. In a description list the next term ends it too. Reading
// the paragraph later, Asciidoctor ends it at a delimiter or an attribute line
// like any other, and at an item line where it reads the paragraph as prose
// ends, see parser.literal, so the lines may hold further blocks, see
// reader.inLiteral.
func (r *itemReader) takeLiteral() {
	from := r.at
	r.take()
	for r.hasMoreLines() {
		kind := r.shape(r.at).kind
		if kind == shapeBlank || kind == shapeContinuation || (r.dlist && r.siblingItem(r.at)) {
			break
		}
		r.take()
	}
	// Not in parser.rb: noted for the gaps in and under it.
	r.literals = append(r.literals, tree.Span{Start: r.p.lines[from].full.Start, End: r.p.lines[r.at-1].full.End})
}

// nestable is which nested lists Asciidoctor looks for.
type nestable int

const (
	// nestableLists is NESTABLE_LIST_CONTEXTS: any list but a callout list.
	nestableLists nestable = iota
	// descriptionLists is [:dlist], which it looks for once within_nested_list.
	descriptionLists
)

// Ruby: within_nested_list ? [:dlist] : NESTABLE_LIST_CONTEXTS
func (r *itemReader) nestable() nestable {
	if r.withinNested {
		return descriptionLists
	}
	return nestableLists
}

// noteNestedList checks whether the current line opens a nested list of the
// kinds given, and notes what Asciidoctor notes when it does: that the item is
// within a nested list, and that a term without text makes it wait for text
// again.
func (r *itemReader) noteNestedList(kinds nestable) bool {
	opens, term := r.nestedAt(r.at, kinds)
	if !opens {
		return false
	}
	// Ruby: within_nested_list = true
	r.withinNested = true
	// Ruby: has_text = false if nested_list_type == :dlist && $3.nil_or_empty?
	if term && r.bareTerm(r.at) {
		r.hasText = false
	}
	return true
}

// noteTermOrItem notes the blank lines above a line within a nested list that
// reads both as a term without text and as an item of another list, so their
// gap keeps its count. Without one above it, Asciidoctor looks for a
// description list only, so the term waits for text; under one, it looks for
// any list first and reads the item, which leaves has_text as it was.
//
// Not in parser.rb.
func (r *itemReader) noteTermOrItem(at int) {
	if !r.withinNested || !r.bareTerm(at) {
		return
	}
	if opens, term := r.nestedAt(at, nestableLists); opens && !term {
		r.note(at, tree.KeepCount)
	}
}

// nestedAt reports whether the line at opens a nested list of the kinds given,
// and whether it does as a term, the one kind whose text Asciidoctor looks at.
func (r *itemReader) nestedAt(at int, kinds nestable) (opens, term bool) {
	sh, ok := listShape(r.text(at), r.p.lines[at].text.Start)
	if kinds == nestableLists && ok && sh.marker != calloutMarker {
		return true, false
	}
	_, ok = termShape(r.p.src, r.p.lines[at])
	return ok, ok
}

// noteAwaitedText notes the gap above the line at while a term still waits for
// text, so it keeps its count. Under a blank line the term takes the line as
// its text, see afterBlank. Without one, Asciidoctor reads a line that opens a
// nested list as that list: from there the item is within a nested list, and a
// term without text waits for text in turn.
//
// Not in parser.rb.
func (r *itemReader) noteAwaitedText(at int) {
	if r.hasText {
		return
	}
	opens, term := r.nestedAt(at, r.nestable())
	if opens && (!r.withinNested || term && r.bareTerm(at)) {
		r.note(at, tree.KeepCount)
	}
}

// bareTerm reports whether the line at is a term with nothing after its
// separator.
func (r *itemReader) bareTerm(at int) bool {
	term, ok := termShape(r.p.src, r.p.lines[at])
	return ok && term.span.End == r.p.lines[at].text.End
}

// siblingItem is is_sibling_list_item?, see siblingAt.
func (r *itemReader) siblingItem(at int) bool {
	_, ok := r.p.siblingAt(at, r.marker)
	return ok
}

// hasMoreLines is reader.has_more_lines?
func (r *itemReader) hasMoreLines() bool { return r.within(r.at) }

// within reports whether the item may still take the line at: the buffer ends
// where the reader around the item does.
func (r *itemReader) within(at int) bool { return !r.p.past(at) }

// shape reads the line the way the buffer holds it: a + an enclosing item
// blanked out is a blank line there.
func (r *itemReader) shape(at int) shape {
	if r.p.reader.blanked(at) {
		return shape{kind: shapeBlank}
	}
	return classify(r.p.src, r.p.lines[at])
}

func (r *itemReader) text(at int) []byte { return r.p.text(r.p.lines[at]) }

// isIndented is LiteralParagraphRx: a line starting with a space or a tab.
// Unlike shapeIndented, it includes an indented list item.
func isIndented(text []byte) bool {
	return len(text) > 0 && (text[0] == ' ' || text[0] == '\t')
}

// bracketed is BlockAttributeLineRx: a block attribute line or a block anchor.
// A block title is not one.
func bracketed(kind shapeKind) bool {
	return kind == shapeAttributes || kind == shapeAnchor
}
