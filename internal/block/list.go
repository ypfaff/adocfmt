package block

import (
	"slices"

	"github.com/ypfaff/adocfmt/internal/tree"
)

// list reads a run of items sharing one marker. AsciiDoc nests on any change of
// marker, not on indentation, so a different marker opens a nested list and a
// marker of an enclosing list ends this one.
func (p *parser) list(b tree.NodeBase, sh shape, opts readOpts) tree.Node {
	start, from := p.pos(), p.at
	list := &tree.List{
		NodeBase:    b,
		Marker:      sh.marker,
		Open:        slices.Clone(opts.lists),
		Section:     p.atSectionLevel(),
		Description: isTermMarker(sh.marker),
	}
	// A + that carries the list does not carry the blocks in its items.
	items := readOpts{
		lists:   append(slices.Clone(opts.lists), sh.marker),
		preWrap: opts.preWrap || hasRole(p.src, b.Meta, preWrapRole),
	}

	gap := tree.Gap{Span: tree.Span{Start: start, End: start}}
	for {
		item := p.listItem(gap, sh, items)
		list.Items = append(list.Items, item)

		m := p.mark()
		gap = p.gap()
		// In a description list the next term ends the literal, see takeLiteral.
		if !list.Description && endsInLiteral(item) {
			gap.Stage = max(gap.Stage, tree.KeepCount)
		}
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

// endsInLiteral reports whether the last block of an item is a literal, looked
// up through the lists nested in it. A literal under a blank line or a + runs
// to the next blank line, an item line included, so without one it would
// swallow the marker of the item below. Without a blank line in the source,
// something else ended the literal, and adding one could end an item around
// the list instead.
func endsInLiteral(item *tree.ListItem) bool {
	if len(item.Children) == 0 {
		return false
	}
	switch last := item.Children[len(item.Children)-1].(type) {
	case *tree.Literal:
		return true
	case *tree.List:
		return endsInLiteral(last.Items[len(last.Items)-1])
	default:
		return false
	}
}

// directiveBelow reports whether a directive stands right under the list. It
// asks past the end of the list, and past the end of the item it stands in,
// because nothing closes one: the items the directive brings in may still be
// items of it.
func (p *parser) directiveBelow() bool {
	at := p.at
	for at < len(p.lines) && classify(p.src, p.lines[at]).kind == shapeBlank {
		at++
	}
	return at < len(p.lines) && classify(p.src, p.lines[at]).kind == shapeDirective
}

// listItem reads the item on the current line in two steps, as Asciidoctor
// does: itemEnd finds the lines the item takes, and the parser reads the
// item's text and blocks from those lines only. opts are the options of the
// item's blocks.
func (p *parser) listItem(gap tree.Gap, sh shape, opts readOpts) *tree.ListItem {
	start := p.pos()
	item := &tree.ListItem{NodeBase: newBase(gap), Marker: sh.span}
	marker := p.at
	bare := sh.span.End == p.lines[marker].text.End

	lines, literals := p.itemEnd(sh, bare)
	p.notes.addLiterals(item, literals)
	p.enter(lines)
	defer p.leave()

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
		p.fold(item, start, opts)
	}
	item.Text = p.itemText(marker, sh.span.End)

	// The text under the item's line is its first block. Comment lines alone
	// are not: Asciidoctor reads them as the metadata of the block below.
	first := opts
	if p.onlyComments(marker + 1) {
		first.textOnly = p.startsTextOnly(marker, sh, bare)
	}
	item.Children = p.attached(first, opts)
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
	for at < p.reader.end && classify(p.src, p.lines[at]).kind == shapeComment {
		at++
	}
	if at == p.reader.end {
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
func (p *parser) fold(item *tree.ListItem, start int, opts readOpts) {
	m := p.mark()
	p.gap()
	if p.done() || !foldsOntoTerm(p.shape().kind) || p.endsItemText(p.shape()) {
		p.rewind(m)
		return
	}

	var folded tree.NodeBase
	p.textRun(&folded, opts, p.endsItemText)
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
// reads as an item of an open list: that one opens a nested list. The first
// block is read with the options first, the others with opts.
//
// Every gap there keeps its count: adjacency is what binds a block to the item,
// and a blank line can push a later block out of it.
func (p *parser) attached(first, opts readOpts) []tree.Node {
	var children []tree.Node
	next := first
	for {
		m := p.mark()
		gap := p.itemGap()
		if p.done() {
			p.rewind(m)
			return children
		}
		node := p.node(gap, next)
		children = append(children, node)
		if _, ok := node.(*tree.Continuation); ok {
			children = append(children, p.carried(p.under(node, next, opts), opts)...)
		}
		next = opts
	}
}

// carried reads the blocks a continuation line attaches to the item. That is
// the block under it. Where attribute entries come first, it is those entries
// and the block under them, because Asciidoctor applies an entry and attaches
// the next block instead. Where a further + follows across a blank line, it is
// that + and the block under it, which Asciidoctor reads under both.
//
// A continuation with nothing left to carry gives the blank lines back, so they
// end up in the gap of the next item or the tail of the enclosing block rather
// than in no node at all.
//
// The first block is read with the options first, the others with opts, see
// under.
func (p *parser) carried(first, opts readOpts) []tree.Node {
	first.carried, opts.carried = true, true
	next := first
	var nodes []tree.Node
	for {
		m := p.mark()
		gap := p.itemGap()
		if p.done() {
			p.rewind(m)
			return nodes
		}
		node := p.node(gap, next)
		nodes = append(nodes, node)
		if !carriesOn(node) {
			return nodes
		}
		next = p.under(node, next, opts)
	}
}

// itemGap reads a gap between the blocks of an item, see attached.
func (p *parser) itemGap() tree.Gap {
	gap := p.gap()
	gap.Stage = max(gap.Stage, tree.KeepCount)
	return gap
}

// under returns the options of the block under node, which was read with
// opts, where item are the options of an item's later blocks. Below comment
// lines that opened the first block, a + is one more line Asciidoctor skips
// among its metadata, so the block under it is the first still, see
// inCommentMeta.
func (p *parser) under(node tree.Node, opts, item readOpts) readOpts {
	if _, ok := node.(*tree.Continuation); ok && p.inCommentMeta() {
		return opts
	}
	return item
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

// carriesOn reports whether a continuation carries the block under the node too.
func carriesOn(node tree.Node) bool {
	switch node.(type) {
	case *tree.Attribute, *tree.Continuation:
		return true
	default:
		return false
	}
}
