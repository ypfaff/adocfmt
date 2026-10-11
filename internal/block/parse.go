// Package block reads AsciiDoc source into the block tree of package tree.
//
// The parser never resolves an include or a conditional, and never guesses.
// Content it cannot know it freezes; structure it cannot trust it reports.
//
// Beside Parse, the package answers how a line reads, for a rule that has to
// ask whether its output still reads the way the parser read the source.
package block

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"unicode/utf8"

	"github.com/ypfaff/adocfmt/internal/tree"
)

var byteOrderMark = []byte{0xEF, 0xBB, 0xBF}

var frontMatterFence = []byte("---")

// Parse cuts src into the block tree.
//
// It fails on input it must not silently repair: anything but UTF-8, and mixed
// line endings.
func Parse(src []byte) (*tree.Document, error) {
	start := bodyStart(src)
	doc := &tree.Document{Src: src, BOM: tree.Span{Start: 0, End: start}}
	if !utf8.Valid(src[start:]) {
		return nil, errors.New("source is not valid UTF-8")
	}
	ending, err := lineEnding(src[start:])
	if err != nil {
		return nil, err
	}
	doc.LineEnding = ending

	p := &parser{src: src, lines: splitLines(src, start), atStart: true, lineBound: map[string]bool{}}
	if matter := p.frontMatter(); matter != nil {
		doc.Nodes = append(doc.Nodes, matter)
	}
	nodes, tail := p.nodes(nil)
	doc.Nodes = append(doc.Nodes, nodes...)
	doc.Tail = tail
	p.freezeItemGaps(doc.Nodes)
	for _, at := range p.regions {
		p.report(at, "conditional region has no endif")
	}

	// An open region is found once the source runs out, so it is reported after
	// findings that stand below it.
	slices.SortStableFunc(p.findings, func(a, b tree.Finding) int { return cmp.Compare(a.Line, b.Line) })

	doc.Findings = p.findings
	return doc, nil
}

// bodyStart is where the source starts behind its byte order mark.
func bodyStart(src []byte) int {
	if bytes.HasPrefix(src, byteOrderMark) {
		return len(byteOrderMark)
	}
	return 0
}

// ReadsAs reports whether lines that replace the node's own still read as a
// node of its kind, so a rule that moves text between lines asks first: a join
// or a split can turn text into syntax. A split can leave a first line that
// reads as an attribute line the node carries, so the node has to hold every
// line itself. The lines are read on their own, without the lists open around
// the node, so a later line that starts like an item of any list is refused as
// well.
func ReadsAs(node tree.Node, lines []byte) bool {
	doc, err := Parse(lines)
	if err != nil || len(doc.Nodes) != 1 {
		return false
	}
	got := doc.Nodes[0]
	if got.Base().Frozen || reflect.TypeOf(got) != reflect.TypeOf(node) || got.Base().Lines != (tree.Span{Start: 0, End: len(lines)}) {
		return false
	}
	for _, line := range Lines(lines)[1:] {
		if _, ok := ListMarker(line); ok {
			return false
		}
	}
	return true
}

// lineEnding reports what ends a line in src, and fails on the endings the
// parser must not repair silently. A source without any answers LF, because a
// rule writing the first line ending has to pick one.
func lineEnding(src []byte) (tree.LineEnding, error) {
	crlf, lf := 0, 0
	for at := 0; at < len(src); at++ {
		switch src[at] {
		case '\n':
			if at > 0 && src[at-1] == '\r' {
				crlf++
			} else {
				lf++
			}
		case '\r':
			if at+1 == len(src) || src[at+1] != '\n' {
				return "", errors.New("source has carriage returns that end no line")
			}
		}
	}
	if crlf > 0 && lf > 0 {
		return "", fmt.Errorf("source has mixed line endings: %d CRLF, %d LF", crlf, lf)
	}
	if crlf > 0 {
		return tree.CRLF, nil
	}
	return tree.LF, nil
}

type parser struct {
	src   []byte
	lines []line
	at    int

	// atStart is true until the first block that cannot precede the document
	// header, so a level 0 title further down stays an ordinary section.
	atStart bool
	// freeze carries the reach of a directive or the front matter fence to the
	// gaps that follow it, see gap.
	freeze bool
	// directiveEnd is where the last directive line the parser read ends, see
	// directiveSince.
	directiveEnd int
	// markers are the list markers of the open lists, innermost last.
	markers []string
	// carrying is true while the parser reads the block a continuation
	// attaches to an item, see endsProse.
	carrying bool
	// regions are the open conditional regions, innermost last, each by the
	// line that opened it. A delimiter that opens in one and closes in another
	// means two documents in one.
	regions []int
	// lineBound holds the document attributes in force that make every line of
	// a paragraph significant. An entry counts from where it stands, inside a
	// conditional too, because the parser resolves none.
	lineBound map[string]bool
	// items are the lines collected for the list items being read, innermost
	// last, see itemEnd.
	items []itemLines
	// notes collects, for the printer, the blank lines and literal
	// paragraphs the items found, see freezeItemGaps.
	notes printerNotes

	findings []tree.Finding
}

func (p *parser) done() bool { return p.at >= p.limit() }

// limit is the line the parser stops at: the end of the innermost list item
// it reads, which ends inside the items around it, or the end of the source.
func (p *parser) limit() int {
	if len(p.items) == 0 {
		return len(p.lines)
	}
	return p.items[len(p.items)-1].end
}

// skipped and blanked report whether the buffer of an item being read leaves
// the line out, or holds the + on it as a blank line. Every one of them counts:
// Asciidoctor reads a nested item from the buffer of its parent, which it read
// from the grandparent's.
func (p *parser) skipped(at int) bool {
	return slices.ContainsFunc(p.items, func(l itemLines) bool { return l.skipped[at] })
}

func (p *parser) blanked(at int) bool {
	return slices.ContainsFunc(p.items, func(l itemLines) bool { return l.blanked == at })
}

// placeholder reports whether the buffer of an item being read holds the + on
// the line as a blank line for the blocks read from it. A nested item reads
// one that stands under a line as a + still, so blanked leaves it out.
func (p *parser) placeholder(at int) bool {
	return slices.ContainsFunc(p.items, func(l itemLines) bool { return l.placeholders[at] })
}

// readsTextOnly reports whether Asciidoctor reads the block starting on the
// line as text only: fewer lines are metadata there, see textOnlyMeta, and
// fewer are blocks, see body.
func (p *parser) readsTextOnly(at int) bool {
	return len(p.items) > 0 && p.items[len(p.items)-1].textOnly == at
}

func (p *parser) pos() int {
	if p.at >= len(p.lines) {
		return len(p.src)
	}
	return p.lines[p.at].full.Start
}

// mark lets the parser look past blank lines and come back when what follows
// them belongs to the next block. Reading the gap consumes the pending freeze,
// so a rewind restores it along with the line. Nothing else changes over blank
// lines, so nothing else is saved.
type mark struct {
	at     int
	freeze bool
}

func (p *parser) mark() mark { return mark{p.at, p.freeze} }

func (p *parser) rewind(m mark) { p.at, p.freeze = m.at, m.freeze }

func (p *parser) shape() shape { return classify(p.src, p.lines[p.at]) }

func (p *parser) text(l line) []byte { return p.src[l.text.Start:l.text.End] }

func (p *parser) take() tree.Span {
	span := p.lines[p.at].full
	p.at++
	return span
}

// closes reports whether the current line ends the enclosing delimited block.
// A delimiter closes only on a line equal to the one that opened it.
func (p *parser) closes(closer []byte) bool {
	return closer != nil && !p.done() && bytes.Equal(p.text(p.lines[p.at]), closer)
}

func (p *parser) gap() tree.Gap {
	start := p.pos()
	for !p.done() && p.shape().kind == shapeBlank {
		p.at++
	}
	gap := tree.Gap{Span: tree.Span{Start: start, End: p.pos()}, Frozen: p.freeze}
	// A freeze reaches to the next blank line of the source rather than to the
	// next node: what a directive brings in, and what the front matter fence
	// opens, may run on into the lines below it. Which of them ends it depends
	// on what it is, but a blank line ends a paragraph whatever it holds, so
	// the freeze follows it that far and every gap it crosses could split it.
	p.freeze = p.freeze && gap.Span.Empty()
	return gap
}

func (p *parser) report(at int, message string) {
	p.findings = append(p.findings, tree.Finding{Line: at + 1, Message: message})
}

// track follows the directive on the current line into or out of a
// conditional region.
func (p *parser) track(sh shape) {
	p.directiveEnd = p.lines[p.at].full.End
	if sh.bad {
		p.report(p.at, "malformed preprocessor directive")
	}
	switch {
	case sh.cond > 0:
		p.regions = append(p.regions, p.at)
	case sh.cond < 0 && len(p.regions) > 0:
		p.regions = p.regions[:len(p.regions)-1]
	case sh.cond < 0:
		p.report(p.at, "endif closes no conditional region")
	}
}

// reach follows the directive on the current line and freezes the block it
// stands in, with the gaps on both sides: what the directive brings in decides
// where that block really starts and ends.
func (p *parser) reach(b *tree.NodeBase, sh shape) {
	p.track(sh)
	b.Frozen, b.Gap.Frozen, p.freeze = true, true, true
}

// directiveSince reports whether the parser has read a directive line at
// offset or later. A block asks it with its own start to learn whether a
// directive stands in its lines, which makes its end uncertain.
func (p *parser) directiveSince(offset int) bool { return p.directiveEnd > offset }

// newBase starts the node below gap. A frozen gap with no blank line in it
// means the reach of a directive or the front matter goes on into this node,
// see gap. What that brings in may continue the node, so it starts frozen.
func newBase(gap tree.Gap) tree.NodeBase {
	return tree.NodeBase{Gap: gap, Frozen: gap.Frozen && gap.Span.Empty()}
}

// region identifies the innermost open conditional region, or -1 outside any.
func (p *parser) region() int {
	if len(p.regions) == 0 {
		return -1
	}
	return p.regions[len(p.regions)-1]
}

func (p *parser) nodes(closer []byte) ([]tree.Node, tree.Gap) {
	var nodes []tree.Node
	for {
		gap := p.gap()
		if p.done() || p.closes(closer) {
			return nodes, gap
		}
		nodes = append(nodes, p.node(gap, closer))
	}
}

// node reads the metadata lines at the current position and the block they
// bind to.
func (p *parser) node(gap tree.Gap, closer []byte) tree.Node {
	b := newBase(gap)
	textOnly := p.readsTextOnly(p.at)
	for p.metaLine(&b, closer, textOnly) {
		b.Gap = p.metaGap()
	}
	return p.block(b, closer, textOnly)
}

// metaGap is the gap under a metadata line. In a list item it takes a + the
// item's buffer holds as a blank line, which Asciidoctor reads past to the
// block under it, as it does a blank line. The gap freezes, since its lines
// are not all blank. Where no block follows, the blank lines under the + go
// back, as they do under a continuation, see carried; block then finds a
// blank line and binds the metadata to nothing.
func (p *parser) metaGap() tree.Gap {
	gap := p.gap()
	for !p.done() && p.placeholder(p.at) {
		p.at++
		m := p.mark()
		p.gap()
		if p.done() {
			p.rewind(m)
		}
		gap = tree.Gap{Span: tree.Span{Start: gap.Span.Start, End: p.pos()}, Frozen: true}
	}
	return gap
}

// metaLine reads the metadata line at the current position into b and reports
// whether there was one.
func (p *parser) metaLine(b *tree.NodeBase, closer []byte, textOnly bool) bool {
	if p.done() || p.closes(closer) {
		return false
	}
	sh := p.shape()
	kind, ok := metaKind(sh)
	if !ok || (len(b.Meta) == 0 && !opensMeta(kind)) || (textOnly && !textOnlyMeta(kind)) {
		return false
	}
	start := p.pos()
	var open bool
	switch kind {
	case tree.MetaDirective:
		p.reach(b, sh)
		p.at++
	case tree.MetaAttrEntry:
		open = p.entry(b, closer)
	case tree.MetaCommentBlock:
		p.commentBlock()
	default:
		p.at++
	}
	b.Meta = append(b.Meta, tree.Meta{Kind: kind, Gap: b.Gap, Lines: tree.Span{Start: start, End: p.pos()}, Open: open})
	return true
}

// opensMeta reports whether a line of this kind starts the metadata of a
// block. The other kinds only continue it, see tree.MetaCommentBlock.
func opensMeta(kind tree.MetaKind) bool {
	switch kind {
	case tree.MetaAttributes, tree.MetaTitle, tree.MetaAnchor, tree.MetaComment:
		return true
	default:
		return false
	}
}

// textOnlyMeta reports whether a line of this kind is metadata where
// Asciidoctor reads a block as text only. A directive counts: Asciidoctor
// resolves it before it reads the line.
func textOnlyMeta(kind tree.MetaKind) bool {
	switch kind {
	case tree.MetaAttributes, tree.MetaAnchor, tree.MetaComment, tree.MetaDirective:
		return true
	default:
		return false
	}
}

// commentBlock skips the comment block opening on the current line.
func (p *parser) commentBlock() {
	openAt := p.at
	closer := p.text(p.lines[p.at])
	p.at++
	p.skipVerbatim(closer, true)
	if p.closingLine().Empty() {
		p.report(openAt, "block has no closing delimiter")
	}
}

// block is the node the metadata lines in b bind to. None follows at the end
// of the source, the enclosing block or the list item, nor where a blank line
// follows: only metaGap leaves one, where the item ends below it.
func (p *parser) block(b tree.NodeBase, closer []byte, textOnly bool) tree.Node {
	if p.done() || p.closes(closer) || p.shape().kind == shapeBlank {
		return p.orphan(b)
	}
	reaches := p.directiveInMeta(b)
	node := p.body(b, closer, textOnly)
	p.freeze = p.freeze || reaches
	return node
}

// orphan holds metadata lines that found no block.
func (p *parser) orphan(b tree.NodeBase) tree.Node {
	b.Lines = tree.Span{Start: p.pos(), End: p.pos()}
	p.freeze = p.freeze || p.directiveInMeta(b)
	return &tree.Opaque{NodeBase: b}
}

// directiveInMeta reports whether a directive stands among the metadata lines.
// What it brings in may reach past the block, so the gap after the block
// freezes too. Ask it before the block is read, or the block's own directives
// count as well.
func (p *parser) directiveInMeta(b tree.NodeBase) bool {
	return p.directiveSince(b.Extent().Start)
}

func metaKind(sh shape) (tree.MetaKind, bool) {
	switch sh.kind {
	case shapeAttributes:
		return tree.MetaAttributes, true
	case shapeTitle:
		return tree.MetaTitle, true
	case shapeAnchor:
		return tree.MetaAnchor, true
	case shapeComment:
		return tree.MetaComment, true
	case shapeAttrEntry:
		return tree.MetaAttrEntry, true
	case shapeDirective:
		return tree.MetaDirective, true
	case shapeDelimiter:
		return tree.MetaCommentBlock, sh.char == '/'
	default:
		return 0, false
	}
}

// body reads the block itself. Section titles exist at section level only:
// inside a delimited block or a list item, Asciidoctor reads a title line as
// prose and a would-be underline as the delimiter it looks like. Only a
// discrete style makes a heading out of the line anywhere.
//
// A level 0 title in either form is the document header while nothing but the
// blocks precedesTitle names came before it, and no block title stands above
// it, see titled.
//
// In a block read as text only, see readsTextOnly, Asciidoctor looks for no
// break, no block macro and no attribute entry: such a line is text. Nor does
// it look for a Markdown quote, a quoted paragraph, an admonition label or a
// literal paragraph, which the parser still reads as Opaque, Admonition or
// Literal; that only costs formatting.
func (p *parser) body(b tree.NodeBase, closer []byte, textOnly bool) tree.Node {
	sh := p.shape()
	if textOnly && (sh.kind == shapeBreak || sh.kind == shapeMacro || sh.kind == shapeAttrEntry) {
		sh.kind = shapeText
	}
	atStart := p.atStart && !titled(p.src, b.Meta)
	p.atStart = atStart && precedesTitle(sh)

	sectionLevel := closer == nil && len(p.markers) == 0
	if sectionLevel && startsTitle(sh.kind) {
		if underline, level, ok := p.setextUnderline(); ok {
			if atStart && level == 0 {
				return p.header(b, p.lines[p.at].text, underline)
			}
			return p.setext(b, level, underline)
		}
	}
	switch sh.kind {
	case shapeDelimiter:
		return p.delimited(b, sh)
	case shapeHeading:
		if atStart && sh.level == 0 {
			return p.header(b, sh.span, p.at)
		}
		if sectionLevel || styledDiscrete(p.src, b.Meta) {
			b.Lines = p.take()
			return &tree.Heading{NodeBase: b, Marker: sh.char, Level: sh.level, Title: sh.span}
		}
		return p.contentBlock(b, sh, closer)
	case shapeAttrEntry:
		return p.attribute(b, closer)
	case shapeDirective:
		return p.directive(b, sh)
	case shapeContinuation:
		// A + carries the block under it only inside a list, and only where no
		// other + carries it already. Under metadata, one that carries is part
		// of the gap, see metaGap; one that reaches here is a line of the block,
		// which is read as it stands, frozen. Outside a list it is a line like
		// any other, which contentBlock reads: text, or code under a verbatim
		// style.
		if !p.inList() {
			return p.contentBlock(b, sh, closer)
		}
		if (p.carrying && b.Gap.Span.Empty()) || len(b.Meta) > 0 {
			return p.plusText(b, closer, endsCarriedPlus)
		}
		b.Gap.Frozen = true
		b.Lines = p.take()
		return &tree.Continuation{NodeBase: b}
	default:
		return p.contentBlock(b, sh, closer)
	}
}

// contentBlock reads a block whose kind a verbatim style above it overrides:
// Asciidoctor checks the style before it looks at the line, and then reads to
// the next blank line whatever the lines look like.
func (p *parser) contentBlock(b tree.NodeBase, sh shape, closer []byte) tree.Node {
	if strictVerbatimStyles[styleOf(p.src, b.Meta)] {
		p.textRun(&b, closer, p.endsVerbatimAfter(p.at))
		return &tree.Literal{NodeBase: b}
	}
	switch sh.kind {
	case shapeContinuation:
		return p.plusText(b, closer, endsText)
	case shapeMarker:
		return p.list(b, sh, closer)
	case shapeIndented:
		return p.literal(b, closer)
	case shapeBreak, shapeMacro:
		b.Lines = p.take()
		return &tree.Opaque{NodeBase: b}
	case shapeQuote:
		p.textRun(&b, closer, p.endsProse(b))
		return &tree.Opaque{NodeBase: b}
	default:
		return p.paragraph(b, closer)
	}
}

// startsTitle reports whether a line of this shape can be the first line of a
// two-line title. Asciidoctor decides the pair before it reads the line as a
// list item, a block macro or a quote; an attribute entry, a directive and the
// shapes carrying no text of their own never form one.
func startsTitle(kind shapeKind) bool {
	switch kind {
	case shapeText, shapeMarker, shapeMacro, shapeQuote, shapeIndented:
		return true
	default:
		return false
	}
}

// precedesTitle reports whether a block of this shape may stand above the
// document title. Asciidoctor reads past an attribute entry, a directive and a
// comment block while it looks for the title; a comment line reaches the title
// as metadata instead, since it binds to the block below it.
func precedesTitle(sh shape) bool {
	switch sh.kind {
	case shapeAttrEntry, shapeDirective:
		return true
	case shapeDelimiter:
		return sh.char == '/'
	default:
		return false
	}
}

// titled reports whether the metadata gives the block a title, as .Title and
// [title=...] do. Asciidoctor reads no document header under one, so a level 0
// title there is a section.
func titled(src []byte, meta []tree.Meta) bool {
	return slices.ContainsFunc(meta, func(m tree.Meta) bool { return m.Kind == tree.MetaTitle }) ||
		hasAttr(src, meta, "title")
}

// header is the document header, which stays compact: it runs to the first
// blank line and takes the author and the revision line under the title, with
// the attribute entries, comment lines and comment blocks among them, which
// Asciidoctor reads without counting. It resolves a directive before it counts,
// so which lines are the author and the revision is unknown then: the header
// freezes and runs to the blank line. underline is the line of the underline
// where the title is written as a title and an underline, and the title line
// itself otherwise.
func (p *parser) header(b tree.NodeBase, title tree.Span, underline int) tree.Node {
	start := p.pos()
	twoLine := underline > p.at
	p.pastTitle(&b, underline)
	titleLines := tree.Span{Start: start, End: p.pos()}
	p.headerBody(&b)
	b.Lines = tree.Span{Start: start, End: p.pos()}
	return &tree.Header{NodeBase: b, Title: title, TitleLines: titleLines, TwoLine: twoLine}
}

// headerBody moves past the lines under the title that belong to the header.
func (p *parser) headerBody(b *tree.NodeBase) {
	for taken := 0; !p.done() && p.shape().kind != shapeBlank; {
		sh := p.shape()
		switch {
		case sh.kind == shapeAttrEntry:
			p.entry(b, nil)
		case sh.kind == shapeDelimiter && sh.char == '/':
			p.commentBlock()
		case sh.kind == shapeDirective:
			p.reach(b, sh)
			p.at++
		case sh.kind == shapeComment:
			p.at++
		case isAuthorOrRevision(taken, p.text(p.lines[p.at])) || p.directiveSince(b.Extent().Start):
			taken++
			p.at++
		default:
			return
		}
	}
}

// isAuthorOrRevision reports whether a line under the title is the author or
// the revision line, where taken counts the lines already read as one of them.
// Asciidoctor takes the author line whatever it says, but the revision line
// only when it matches RevisionInfoLineRx.
func isAuthorOrRevision(taken int, text []byte) bool {
	switch taken {
	case 0:
		return true
	case 1:
		return isRevisionLine(text)
	default:
		return false
	}
}

// isRevisionLine mirrors Asciidoctor's RevisionInfoLineRx, which fails only
// where the line and the text after each of its commas all start with a colon,
// as :x and :x,:y do.
func isRevisionLine(text []byte) bool {
	for field := range bytes.SplitSeq(text, []byte(",")) {
		if !bytes.HasPrefix(field, []byte(":")) {
			return true
		}
	}
	return false
}

func (p *parser) attribute(b tree.NodeBase, closer []byte) tree.Node {
	start := p.pos()
	p.entry(&b, closer)
	b.Lines = tree.Span{Start: start, End: p.pos()}
	return &tree.Attribute{NodeBase: b}
}

// entry reads the attribute entry on the current line with the lines that
// continue its value. Asciidoctor takes every line up to a blank one for as
// long as the line before ends in the entry's own wrap marker, whatever the
// line looks like. A directive among them it resolves first, so the entry
// freezes the way a paragraph around a directive does. Inside a delimited
// block Asciidoctor reads only the lines up to the closing delimiter, so the
// value ends there too.
//
// It reports whether a blank line is what ended the value, see tree.Meta.Open.
func (p *parser) entry(b *tree.NodeBase, closer []byte) bool {
	e, _ := parseAttrEntry(p.text(p.lines[p.at]))
	p.bind(e)
	p.at++
	for open := e.wrap != ""; open && !p.done() && !p.closes(closer); p.at++ {
		sh := p.shape()
		if sh.kind == shapeBlank {
			return true
		}
		if sh.kind == shapeDirective {
			p.reach(b, sh)
			continue
		}
		open = bytes.HasSuffix(bytes.TrimLeft(p.text(p.lines[p.at]), " \t"), []byte(e.wrap))
	}
	return false
}

// bind follows an attribute entry into or out of lineBound. hardbreaks-option
// renders every line break, under its old name hardbreaks too; attribute-missing
// drops a whole line with an unresolved reference, but only when set to
// drop-line.
func (p *parser) bind(e attrEntry) {
	on := e.set
	switch e.name {
	case "hardbreaks", "hardbreaks-option":
		e.name = "hardbreaks-option"
	case "attribute-missing":
		on = on && e.value == "drop-line"
	default:
		return
	}
	if on {
		p.lineBound[e.name] = true
	} else {
		delete(p.lineBound, e.name)
	}
}

// setextUnderline finds the underline of a two-line title starting on the
// current line, and its level. Asciidoctor resolves the conditionals between
// the two lines before it pairs them. Only those that open a region are looked
// past: the underline then shows only where the title line shows too, right
// under it.
func (p *parser) setextUnderline() (underline, level int, ok bool) {
	underline = p.at + 1
	for underline < p.limit() && opensRegion(classify(p.src, p.lines[underline])) {
		underline++
	}
	if underline >= p.limit() {
		return 0, 0, false
	}
	level, ok = setextLevel(p.src, p.lines[p.at], p.lines[underline])
	return underline, level, ok
}

func opensRegion(sh shape) bool { return sh.kind == shapeDirective && sh.cond > 0 }

// pastTitle moves past a title down to its underline. A conditional between
// the two freezes the title and the blocks below it down to the next blank
// line: where it drops the underline, the title line is prose that runs on
// into them.
func (p *parser) pastTitle(b *tree.NodeBase, underline int) {
	for p.at++; p.at <= underline; p.at++ {
		if sh := p.shape(); sh.kind == shapeDirective {
			p.reach(b, sh)
		}
	}
}

// setext reads a two-line title. One whose title line reads as a list item
// stays as it stands: a description list above keeps the block attribute
// lines over such a line, so the one-line form would hand them to the title.
func (p *parser) setext(b tree.NodeBase, level, underline int) tree.Node {
	title := p.lines[p.at].text
	b.Frozen = b.Frozen || p.shape().kind == shapeMarker
	start := p.pos()
	p.pastTitle(&b, underline)
	b.Lines = tree.Span{Start: start, End: p.pos()}
	return &tree.Setext{NodeBase: b, Level: level, Title: title}
}

// paragraph reads prose, unless its line breaks carry meaning: a verbatim
// style, the hardbreaks option, substitutions of its own, or a document
// attribute in force that binds lines makes it a Literal. A quoted paragraph
// is Opaque like the Markdown quote, since Asciidoctor lifts its last line out
// as the attribution. So is one with a style or a block macro on its first
// line that only an extension knows, see extensionStyle and
// isCustomBlockMacro. A label opening its first line makes it an Admonition.
func (p *parser) paragraph(b tree.NodeBase, closer []byte) tree.Node {
	first := p.at
	p.textRun(&b, closer, p.endsProse(b))
	style := styleOf(p.src, b.Meta)
	switch {
	case !paragraphStyles[style] && quotedParagraph(p.src, p.lines[first:p.at]),
		extensionStyle(style) || isCustomBlockMacro(p.text(p.lines[first])):
		return &tree.Opaque{NodeBase: b}
	case styledVerbatim(p.src, b.Meta) || hasOption(p.src, b.Meta, hardbreaksOption) ||
		hasAttr(p.src, b.Meta, substitutionsAttr) || len(p.lineBound) > 0:
		return &tree.Literal{NodeBase: b}
	case admonition(p.text(p.lines[first])):
		return &tree.Admonition{NodeBase: b}
	default:
		return &tree.Paragraph{NodeBase: b}
	}
}

// admonitionLabels are the labels that open an admonition paragraph.
var admonitionLabels = []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"}

// admonition reports whether a paragraph's first line opens with an admonition
// label, a colon and whitespace, as Asciidoctor requires.
func admonition(line []byte) bool {
	for _, label := range admonitionLabels {
		rest, ok := bytes.CutPrefix(line, []byte(label+":"))
		if ok && len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
			return true
		}
	}
	return false
}

// literal is an indented paragraph, which AsciiDoc reads as verbatim content.
func (p *parser) literal(b tree.NodeBase, closer []byte) tree.Node {
	p.textRun(&b, closer, endsText)
	return &tree.Literal{NodeBase: b}
}

// textRun consumes the lines of an undelimited block up to the first line ends
// says is no longer part of it. A directive line inside the block does not end
// it but freezes it, see reach. A line that has to stay a line of its own
// freezes the block too, see pinsLine.
func (p *parser) textRun(b *tree.NodeBase, closer []byte, ends func(shape) bool) {
	start := p.pos()
	for !p.done() && !p.closes(closer) {
		sh := p.shape()
		if ends(sh) {
			break
		}
		if sh.kind == shapeDirective {
			p.reach(b, sh)
		}
		b.Frozen = b.Frozen || pinsLine(sh, p.text(p.lines[p.at]), p.inList())
		p.at++
	}
	b.Lines = tree.Span{Start: start, End: p.pos()}
}

// endsText reports whether a line of this shape ends the text of a paragraph
// or a list item. Asciidoctor stops at a blank line, a delimiter, a block
// attribute or anchor line and a lone +, and reads on past a heading line or a
// comment.
func endsText(sh shape) bool {
	switch sh.kind {
	case shapeBlank, shapeDelimiter, shapeAttributes, shapeAnchor, shapeContinuation:
		return true
	default:
		return false
	}
}

// endsItemText also stops at the next item, which keeps a term from being read
// as the text of the item above it.
func endsItemText(sh shape) bool {
	return endsText(sh) || sh.kind == shapeMarker
}

// endsCarriedPlus is where the paragraph ends that a + under a carrying + opens.
// It stops where any text does, except at a further lone +, which Asciidoctor
// drops out of the paragraph rather than ending it on.
func endsCarriedPlus(sh shape) bool {
	return endsText(sh) && sh.kind != shapeContinuation
}

// endsVerbatim stops where Asciidoctor stops a verbatim-styled paragraph: at a
// blank line or a lone +, whatever the lines in between look like.
func endsVerbatim(sh shape) bool {
	return sh.kind == shapeBlank || sh.kind == shapeContinuation
}

// endsVerbatimAfter is endsVerbatim for a paragraph whose first line is first.
// Asciidoctor ends it at a lone + only once it has read a line, so a + on the
// first line is code.
func (p *parser) endsVerbatimAfter(first int) func(shape) bool {
	return func(sh shape) bool { return p.at > first && endsVerbatim(sh) }
}

// endsProse is where a paragraph with the metadata in b ends at the current
// position. Inside an item Asciidoctor breaks a paragraph at an item line only
// when no blank line came before it, and a continuation counts as one: a
// paragraph right after the item's text ends at any item line, a carried one
// does not. Nor does one with a paragraph style, which Asciidoctor reads as
// the block that style names.
// A literal paragraph does not use it: Asciidoctor reads that up to a blank
// line, an item line included.
func (p *parser) endsProse(b tree.NodeBase) func(shape) bool {
	if p.inList() && !p.carrying && !paragraphStyles[styleOf(p.src, b.Meta)] {
		return endsItemText
	}
	return endsText
}

// plusText reads a + that carries nothing, with the lines under it that
// Asciidoctor reads as one paragraph with it. The + renders as the text it is,
// and a further one among those lines Asciidoctor drops, so what the paragraph
// shows is not the lines that stand here and none of them may move.
func (p *parser) plusText(b tree.NodeBase, closer []byte, ends func(shape) bool) tree.Node {
	start := p.pos()
	p.at++
	p.textRun(&b, closer, ends)
	b.Frozen, b.Gap.Frozen = true, true
	b.Lines = tree.Span{Start: start, End: p.pos()}
	return &tree.Paragraph{NodeBase: b}
}

func (p *parser) directive(b tree.NodeBase, sh shape) tree.Node {
	p.reach(&b, sh)
	b.Lines = p.take()
	return &tree.Directive{NodeBase: b}
}

func (p *parser) delimited(b tree.NodeBase, sh shape) tree.Node {
	openAt := p.at
	open := p.lines[p.at]
	// A fence may carry a language after the backticks; only the bare fence
	// closes it. Every other delimiter closes on a line equal to its opener.
	closer := p.text(open)[:sh.width]
	region := p.region()
	start := p.pos()
	delim := tree.Delimiter{Char: sh.char, Fill: sh.fill, Width: sh.width, Open: open.full}
	style := styleOf(p.src, b.Meta)
	raw := readsRaw(sh, style)
	if sh.content == contentCompound && (verbatimStyles[style] || style == commentStyle || extensionStyle(style)) {
		sh.content = contentVerbatim
	}
	p.at++

	var node tree.Node
	if sh.content == contentCompound {
		// The block confines the reader: no list open outside it is open inside.
		markers := p.markers
		p.markers = nil
		children, tail := p.nodes(closer)
		p.markers = markers
		delim.Extensible = p.extensible(raw, openAt+1)
		if at, ok := p.closesInside(closer, openAt+1); ok {
			p.report(at, "closes the block opened above it")
		}
		delim.Close = p.closingLine()
		b.Lines = tree.Span{Start: start, End: p.pos()}
		node = &tree.Container{NodeBase: b, Delim: delim, Children: children, Tail: tail}
	} else {
		p.skipVerbatim(closer, raw)
		delim.Extensible = p.extensible(raw, openAt+1)
		delim.Close = p.closingLine()
		b.Lines = tree.Span{Start: start, End: p.pos()}
		if sh.content == contentTable {
			node = &tree.Table{NodeBase: b, Delim: delim}
		} else {
			node = &tree.Verbatim{NodeBase: b, Delim: delim}
		}
	}

	if !delim.Closed() {
		p.report(openAt, "block has no closing delimiter")
	}
	if p.region() != region {
		p.report(openAt, "delimiter opens and closes in different conditional regions")
	}
	return node
}

// readsRaw reports whether Asciidoctor reads the block with the preprocessor
// off, so no directive inside it opens a conditional region or brings a line in.
// That is the comment block, and the open block a [comment] turns into one. On
// any wider fence Asciidoctor drops the style and reads the block its delimiter
// names.
func readsRaw(sh shape, style string) bool {
	return sh.char == '/' || (style == commentStyle && sh.width == openBlockWidth)
}

// extensible reports whether a directive stands between from and the line the
// block closes on. What one brings in is known only once Asciidoctor has
// resolved it, so the body may hold lines the parser never sees, a line
// closing the block among them. A block read raw holds no such line, see
// readsRaw.
func (p *parser) extensible(raw bool, from int) bool {
	if raw {
		return false
	}
	for at := from; at < p.at; at++ {
		if classify(p.src, p.lines[at]).kind == shapeDirective {
			return true
		}
	}
	return false
}

// skipVerbatim advances to the line closing a verbatim block, tracking the
// conditional regions a directive inside it opens, see readsRaw.
func (p *parser) skipVerbatim(closer []byte, raw bool) {
	for !p.done() && !bytes.Equal(p.text(p.lines[p.at]), closer) {
		if inner := p.shape(); inner.kind == shapeDirective && !raw {
			p.track(inner)
		}
		p.at++
	}
}

// closesInside finds a line of the body equal to the one that opened the block.
// Asciidoctor ends a block on the first such line, however deep the nodes read
// so far put it, so the tree holds blocks the document does not have. Which
// reading was meant is not the parser's to guess.
//
// A verbatim block and a table cannot hold one, since skipVerbatim stops there.
func (p *parser) closesInside(closer []byte, from int) (int, bool) {
	for at := from; at < p.at; at++ {
		if bytes.Equal(p.text(p.lines[at]), closer) {
			return at, true
		}
	}
	return 0, false
}

func (p *parser) closingLine() tree.Span {
	if p.done() {
		return tree.Span{}
	}
	return p.take()
}

// frontMatter reads the YAML block a static site generator puts first. It is
// detected on the raw input because AsciiDoc would read it as a thematic break
// plus prose and reformat it.
//
// Asciidoctor itself does not skip it without skip-front-matter: it reads the
// opening fence as a thematic break and everything under it up to the first
// blank line as one paragraph. That is why the block freezes what follows.
func (p *parser) frontMatter() tree.Node {
	start := p.pos()
	for !p.done() && p.shape().kind == shapeBlank {
		p.at++
	}
	if p.done() || !p.onFence() {
		p.at = 0
		return nil
	}
	if p.pos() > start {
		// The fence opens front matter on the first line only, so the blank
		// lines above it are what keeps this document from having any. They
		// stay whether or not a closing fence follows, since a rule may move
		// that line and the next run would decide otherwise.
		p.at, p.freeze = 0, true
		return nil
	}
	p.at++
	for !p.done() && !p.onFence() {
		p.at++
	}
	if p.done() {
		p.at = 0
		return nil
	}
	p.at++
	p.freeze = true
	b := tree.NodeBase{Gap: tree.Gap{Span: tree.Span{Start: start, End: start}}, Lines: tree.Span{Start: start, End: p.pos()}}
	return &tree.FrontMatter{NodeBase: b}
}

func (p *parser) onFence() bool {
	return bytes.Equal(p.text(p.lines[p.at]), frontMatterFence)
}
