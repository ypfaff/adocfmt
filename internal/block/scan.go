package block

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"unicode/utf8"
)

var byteOrderMark = []byte{0xEF, 0xBB, 0xBF}

var frontMatterFence = []byte("---")

// Scan cuts src into the block tree.
//
// It fails on input it must not silently repair: anything but UTF-8, and mixed
// line endings.
func Scan(src []byte) (*Document, error) {
	doc := &Document{Src: src}
	start := 0
	if bytes.HasPrefix(src, byteOrderMark) {
		doc.BOM = Span{0, len(byteOrderMark)}
		start = len(byteOrderMark)
	}
	if !utf8.Valid(src[start:]) {
		return nil, errors.New("source is not valid UTF-8")
	}
	ending, err := lineEnding(src[start:])
	if err != nil {
		return nil, err
	}
	doc.LineEnding = ending

	s := &scanner{src: src, lines: splitLines(src, start), atStart: true, lineBound: map[string]bool{}}
	if matter := s.frontMatter(); matter != nil {
		doc.Nodes = append(doc.Nodes, matter)
	}
	nodes, tail := s.nodes(nil)
	doc.Nodes = append(doc.Nodes, nodes...)
	doc.Tail = tail
	for _, at := range s.regions {
		s.report(at, "conditional region has no endif")
	}

	// An open region is found once the source runs out, so it is reported after
	// findings that stand below it.
	slices.SortStableFunc(s.findings, func(a, b Finding) int { return cmp.Compare(a.Line, b.Line) })

	doc.Findings = s.findings
	return doc, nil
}

// ReadsAs reports whether lines that replace the node's own still read as a
// node of its kind, so a rule that moves text between lines asks first: a join
// or a split can turn text into syntax. A split can leave a first line that
// reads as an attribute line the node carries, so the node has to hold every
// line itself. The lines are read on their own, without the lists open around
// the node, so a later line that starts like an item of any list is refused as
// well.
func ReadsAs(node Node, lines []byte) bool {
	doc, err := Scan(lines)
	if err != nil || len(doc.Nodes) != 1 {
		return false
	}
	got := doc.Nodes[0]
	if got.Frozen() || reflect.TypeOf(got) != reflect.TypeOf(node) || got.Lines() != (Span{0, len(lines)}) {
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
// scanner must not repair silently. A source without any answers LF, because a
// rule writing the first line ending has to pick one.
func lineEnding(src []byte) (LineEnding, error) {
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
		return CRLF, nil
	}
	return LF, nil
}

type scanner struct {
	src   []byte
	lines []line
	at    int

	// atStart is true until the first block that cannot precede the document
	// header, so a level 0 title further down stays an ordinary section.
	atStart bool
	// freeze carries the reach of a directive or the front matter fence to the
	// gaps that follow it, see gap.
	freeze bool
	// markers are the list markers of the open lists, innermost last.
	markers []string
	// carrying is true while the scanner reads the block a continuation
	// attaches to an item, see endsProse.
	carrying bool
	// regions are the open conditional regions, innermost last, each by the
	// line that opened it. A delimiter that opens in one and closes in another
	// means two documents in one.
	regions []int
	// lineBound holds the document attributes in force that make every line of
	// a paragraph significant. An entry counts from where it stands, inside a
	// conditional too, because the scanner resolves none.
	lineBound map[string]bool

	findings []Finding
}

func (s *scanner) done() bool { return s.at >= len(s.lines) }

func (s *scanner) pos() int {
	if s.done() {
		return len(s.src)
	}
	return s.lines[s.at].full.Start
}

// mark lets the scanner look past blank lines and come back when what follows
// them belongs to the next block. Reading the gap consumes the pending freeze,
// so a rewind restores it along with the line. Nothing else changes over blank
// lines, so nothing else is saved.
type mark struct {
	at     int
	freeze bool
}

func (s *scanner) mark() mark { return mark{s.at, s.freeze} }

func (s *scanner) rewind(m mark) { s.at, s.freeze = m.at, m.freeze }

func (s *scanner) shape() shape { return classify(s.src, s.lines[s.at]) }

func (s *scanner) text(l line) []byte { return s.src[l.text.Start:l.text.End] }

func (s *scanner) take() Span {
	span := s.lines[s.at].full
	s.at++
	return span
}

// closes reports whether the current line ends the enclosing delimited block.
// A delimiter closes only on a line equal to the one that opened it.
func (s *scanner) closes(closer []byte) bool {
	return closer != nil && !s.done() && bytes.Equal(s.text(s.lines[s.at]), closer)
}

func (s *scanner) gap() Gap {
	start := s.pos()
	for !s.done() && s.shape().kind == shapeBlank {
		s.at++
	}
	gap := Gap{Span: Span{start, s.pos()}, Frozen: s.freeze}
	// A freeze reaches to the next blank line of the source rather than to the
	// next node: what a directive brings in, and what the front matter fence
	// opens, Asciidoctor reads on as one block until a blank line ends it, so
	// every gap it crosses on the way would split it.
	s.freeze = s.freeze && gap.Span.Empty()
	return gap
}

func (s *scanner) report(at int, message string) {
	s.findings = append(s.findings, Finding{Line: at + 1, Message: message})
}

// track follows the directive on the current line into or out of a
// conditional region.
func (s *scanner) track(sh shape) {
	if sh.bad {
		s.report(s.at, "malformed preprocessor directive")
	}
	switch {
	case sh.cond > 0:
		s.regions = append(s.regions, s.at)
	case sh.cond < 0 && len(s.regions) > 0:
		s.regions = s.regions[:len(s.regions)-1]
	case sh.cond < 0:
		s.report(s.at, "endif closes no conditional region")
	}
}

// region identifies the innermost open conditional region, or -1 outside any.
func (s *scanner) region() int {
	if len(s.regions) == 0 {
		return -1
	}
	return s.regions[len(s.regions)-1]
}

func (s *scanner) nodes(closer []byte) ([]Node, Gap) {
	var nodes []Node
	for {
		gap := s.gap()
		if s.done() || s.closes(closer) {
			return nodes, gap
		}
		nodes = append(nodes, s.node(gap, closer))
	}
}

// node reads the metadata lines at the current position and the block they
// bind to.
func (s *scanner) node(gap Gap, closer []byte) Node {
	b := base{gap: gap}
	for s.metaLine(&b, closer) {
		b.gap = s.gap()
	}
	return s.block(b, closer)
}

// metaLine reads the metadata line at the current position into b and reports
// whether there was one.
func (s *scanner) metaLine(b *base, closer []byte) bool {
	if s.done() || s.closes(closer) {
		return false
	}
	sh := s.shape()
	kind, ok := metaKind(sh)
	if !ok || (len(b.meta) == 0 && !opensMeta(kind)) {
		return false
	}
	start := s.pos()
	var open bool
	switch kind {
	case MetaDirective:
		s.track(sh)
		b.frozen, b.gap.Frozen, s.freeze = true, true, true
		s.at++
	case MetaAttrEntry:
		open = s.entry(b)
	case MetaCommentBlock:
		s.commentBlock()
	default:
		s.at++
	}
	b.meta = append(b.meta, Meta{Kind: kind, Gap: b.gap, Lines: Span{start, s.pos()}, Open: open})
	return true
}

// opensMeta reports whether a line of this kind starts the metadata of a
// block. The other kinds only continue it, see MetaCommentBlock.
func opensMeta(kind MetaKind) bool {
	switch kind {
	case MetaAttributes, MetaTitle, MetaAnchor, MetaComment:
		return true
	default:
		return false
	}
}

// commentBlock skips the comment block opening on the current line.
func (s *scanner) commentBlock() {
	openAt := s.at
	closer := s.text(s.lines[s.at])
	s.at++
	s.skipVerbatim(closer, true)
	if s.closingLine().Empty() {
		s.report(openAt, "block has no closing delimiter")
	}
}

// block is the node the metadata lines in b bind to. None follows at the end
// of the source or the enclosing block, or at an item of an open list, which
// ends the item being read whatever came before it.
func (s *scanner) block(b base, closer []byte) Node {
	if s.done() || s.closes(closer) || (len(b.meta) > 0 && s.sibling()) {
		return s.orphan(b)
	}
	node := s.body(b, closer)
	s.freezeAfter(b)
	return node
}

// orphan holds metadata lines that found no block.
func (s *scanner) orphan(b base) Node {
	b.lines = Span{s.pos(), s.pos()}
	s.freezeAfter(b)
	return &Opaque{base: b}
}

// freezeAfter carries a directive among the metadata lines to the gap after the
// block, past whatever the block itself read.
func (s *scanner) freezeAfter(b base) {
	if b.frozen {
		s.freeze = true
	}
}

func metaKind(sh shape) (MetaKind, bool) {
	switch sh.kind {
	case shapeAttributes:
		return MetaAttributes, true
	case shapeTitle:
		return MetaTitle, true
	case shapeAnchor:
		return MetaAnchor, true
	case shapeComment:
		return MetaComment, true
	case shapeAttrEntry:
		return MetaAttrEntry, true
	case shapeDirective:
		return MetaDirective, true
	case shapeDelimiter:
		return MetaCommentBlock, sh.char == '/'
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
// blocks precedesTitle names came before it.
func (s *scanner) body(b base, closer []byte) Node {
	sh := s.shape()
	atStart := s.atStart
	s.atStart = atStart && precedesTitle(sh)

	sectionLevel := closer == nil && len(s.markers) == 0
	if sectionLevel && startsTitle(sh.kind) {
		if level, ok := s.setextLevel(); ok {
			if atStart && level == 0 {
				return s.header(b, s.lines[s.at].text, true)
			}
			return s.setext(b, level)
		}
	}
	switch sh.kind {
	case shapeDelimiter:
		return s.delimited(b, sh)
	case shapeHeading:
		if atStart && sh.level == 0 {
			return s.header(b, sh.span, false)
		}
		if sectionLevel || styledDiscrete(s.src, b.meta) {
			b.lines = s.take()
			return &Heading{base: b, Marker: sh.char, Level: sh.level, Title: sh.span}
		}
		return s.contentBlock(b, sh, closer)
	case shapeAttrEntry:
		return s.attribute(b)
	case shapeDirective:
		return s.directive(b, sh)
	case shapeContinuation:
		// A + carries the block under it only inside a list, and only where no
		// other + carries it already. Everywhere else it is text again.
		if !s.inList() {
			return s.plusText(b, closer, endsText)
		}
		if s.carrying && b.gap.Span.Empty() {
			return s.plusText(b, closer, endsCarriedPlus)
		}
		b.gap.Frozen = true
		b.lines = s.take()
		return &Continuation{base: b}
	default:
		return s.contentBlock(b, sh, closer)
	}
}

// contentBlock reads a block whose kind a verbatim style above it overrides:
// Asciidoctor checks the style before it looks at the line, and then reads to
// the next blank line whatever the lines look like.
func (s *scanner) contentBlock(b base, sh shape, closer []byte) Node {
	if strictVerbatimStyles[styleOf(s.src, b.meta)] {
		s.textRun(&b, closer, endsVerbatim)
		return &Literal{base: b}
	}
	switch sh.kind {
	case shapeMarker:
		return s.list(b, sh, closer)
	case shapeIndented:
		return s.literal(b, closer)
	case shapeBreak, shapeMacro:
		b.lines = s.take()
		return &Opaque{base: b}
	case shapeQuote:
		s.textRun(&b, closer, s.endsProse())
		return &Opaque{base: b}
	default:
		return s.paragraph(b, closer)
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

// headerLines is what the document title takes under it: the author line and
// the revision line. Asciidoctor counts the two rather than reading what they
// say, so whatever stands there belongs to the header.
const headerLines = 2

// header is the document header, which stays compact: it runs to the first
// blank line and takes headerLines under the title, with the attribute and
// comment lines among them, which Asciidoctor reads without counting. twoLine
// says the title is written as a title and an underline.
func (s *scanner) header(b base, title Span, twoLine bool) Node {
	start := s.pos()
	s.at++
	if twoLine {
		s.at++
	}
	titleLines := Span{start, s.pos()}
	taken := 0
	for !s.done() && s.shape().kind != shapeBlank {
		kind := s.shape().kind
		if kind == shapeAttrEntry {
			s.entry(&b)
			continue
		}
		if kind != shapeComment {
			if taken == headerLines {
				break
			}
			taken++
		}
		s.at++
	}
	b.lines = Span{start, s.pos()}
	return &Header{base: b, Title: title, TitleLines: titleLines, TwoLine: twoLine}
}

func (s *scanner) attribute(b base) Node {
	start := s.pos()
	s.entry(&b)
	b.lines = Span{start, s.pos()}
	return &Attribute{base: b}
}

// entry reads the attribute entry on the current line with the lines that
// continue its value. Asciidoctor takes every line up to a blank one for as
// long as the line before ends in the entry's own wrap marker, whatever the
// line looks like. A directive among them it resolves first, so the entry
// freezes the way a paragraph around a directive does.
//
// It reports whether a blank line is what ended the value, see Meta.Open.
func (s *scanner) entry(b *base) bool {
	e, _ := parseAttrEntry(s.text(s.lines[s.at]))
	s.bind(e)
	s.at++
	for open := e.wrap != ""; open && !s.done(); s.at++ {
		sh := s.shape()
		if sh.kind == shapeBlank {
			return true
		}
		if sh.kind == shapeDirective {
			s.track(sh)
			b.frozen, b.gap.Frozen, s.freeze = true, true, true
			continue
		}
		open = bytes.HasSuffix(bytes.TrimLeft(s.text(s.lines[s.at]), " \t"), []byte(e.wrap))
	}
	return false
}

// bind follows an attribute entry into or out of lineBound. hardbreaks-option
// renders every line break, under its old name hardbreaks too; attribute-missing
// drops a whole line with an unresolved reference, but only when set to
// drop-line.
func (s *scanner) bind(e attrEntry) {
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
		s.lineBound[e.name] = true
	} else {
		delete(s.lineBound, e.name)
	}
}

// setextLevel reports whether the current line and the next form a two-line
// title, and its level.
func (s *scanner) setextLevel() (int, bool) {
	if s.at+1 >= len(s.lines) {
		return 0, false
	}
	return setextLevel(s.src, s.lines[s.at], s.lines[s.at+1])
}

func (s *scanner) setext(b base, level int) Node {
	title := s.lines[s.at].text
	start := s.pos()
	s.at += 2
	b.lines = Span{start, s.pos()}
	return &Setext{base: b, Level: level, Title: title}
}

// paragraph reads prose, unless its line breaks carry meaning: a verbatim
// style, the hardbreaks option, substitutions of its own, or a document
// attribute in force that binds lines makes it a Literal. A quoted paragraph
// is Opaque like the Markdown quote, since Asciidoctor lifts its last line out
// as the attribution. A label opening its first line makes it an Admonition.
func (s *scanner) paragraph(b base, closer []byte) Node {
	first := s.at
	s.textRun(&b, closer, s.endsProse())
	switch {
	case !paragraphStyles[styleOf(s.src, b.meta)] && quotedParagraph(s.src, s.lines[first:s.at]):
		return &Opaque{base: b}
	case styledVerbatim(s.src, b.meta) || hasOption(s.src, b.meta, hardbreaksOption) ||
		hasAttr(s.src, b.meta, substitutionsAttr) || len(s.lineBound) > 0:
		return &Literal{base: b}
	case admonition(s.text(s.lines[first])):
		return &Admonition{base: b}
	default:
		return &Paragraph{base: b}
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
func (s *scanner) literal(b base, closer []byte) Node {
	s.textRun(&b, closer, endsText)
	return &Literal{base: b}
}

// textRun consumes the lines of an undelimited block up to the first line ends
// says is no longer part of it. A directive line inside the block does not end
// it: what the directive pulls in decides where the block really ends, so the
// block and the gaps around it freeze instead. A line that has to stay a line
// of its own freezes the block too, see pinsLine.
func (s *scanner) textRun(b *base, closer []byte, ends func(shape) bool) {
	start := s.pos()
	directive := false
	for !s.done() && !s.closes(closer) {
		sh := s.shape()
		if ends(sh) {
			break
		}
		if sh.kind == shapeDirective {
			s.track(sh)
			directive = true
		}
		b.frozen = b.frozen || sh.kind == shapeDirective || pinsLine(sh, s.text(s.lines[s.at]))
		s.at++
	}
	b.lines = Span{start, s.pos()}
	if directive {
		b.gap.Frozen = true
		s.freeze = true
	}
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

// endsProse is where a paragraph ends at the current position. Inside an item
// Asciidoctor breaks a paragraph at an item line only when no blank line came
// before it, and a continuation counts as one: a paragraph right after the
// item's text ends at any item line, a carried one at an item of an open list.
// A literal paragraph does not use it: Asciidoctor reads that up to a blank
// line, an item line included.
func (s *scanner) endsProse() func(shape) bool {
	switch {
	case !s.inList():
		return endsText
	case s.carrying:
		return s.endsCarriedText
	default:
		return endsItemText
	}
}

func (s *scanner) endsCarriedText(sh shape) bool {
	return endsText(sh) || s.sibling()
}

// plusText reads a + that carries nothing, with the lines under it that
// Asciidoctor reads as one paragraph with it. The + renders as the text it is,
// and a further one among those lines Asciidoctor drops, so what the paragraph
// shows is not the lines that stand here and none of them may move.
func (s *scanner) plusText(b base, closer []byte, ends func(shape) bool) Node {
	start := s.pos()
	s.at++
	s.textRun(&b, closer, ends)
	b.frozen, b.gap.Frozen = true, true
	b.lines = Span{start, s.pos()}
	return &Paragraph{base: b}
}

func (s *scanner) directive(b base, sh shape) Node {
	s.track(sh)
	b.gap.Frozen = true
	b.frozen = true
	b.lines = s.take()
	s.freeze = true
	return &Directive{base: b}
}

func (s *scanner) delimited(b base, sh shape) Node {
	openAt := s.at
	open := s.lines[s.at]
	// A fence may carry a language after the backticks; only the bare fence
	// closes it. Every other delimiter closes on a line equal to its opener.
	closer := s.text(open)[:sh.width]
	region := s.region()
	start := s.pos()
	delim := Delimiter{Char: sh.char, Fill: sh.fill, Width: sh.width, Open: open.full}
	style := styleOf(s.src, b.meta)
	raw := readsRaw(sh, style)
	if sh.content == contentCompound && (verbatimStyles[style] || style == commentStyle) {
		sh.content = contentVerbatim
	}
	s.at++

	var node Node
	if sh.content == contentCompound {
		// The block confines the reader: no list open outside it is open inside.
		markers := s.markers
		s.markers = nil
		children, tail := s.nodes(closer)
		s.markers = markers
		delim.Extensible = s.extensible(raw, openAt+1)
		if at, ok := s.closesInside(closer, openAt+1); ok {
			s.report(at, "closes the block opened above it")
		}
		delim.Close = s.closingLine()
		b.lines = Span{start, s.pos()}
		node = &Container{base: b, Delim: delim, Children: children, Tail: tail}
	} else {
		s.skipVerbatim(closer, raw)
		delim.Extensible = s.extensible(raw, openAt+1)
		delim.Close = s.closingLine()
		b.lines = Span{start, s.pos()}
		if sh.content == contentTable {
			node = &Table{base: b, Delim: delim}
		} else {
			node = &Verbatim{base: b, Delim: delim}
		}
	}

	if !delim.Closed() {
		s.report(openAt, "block has no closing delimiter")
	}
	if s.region() != region {
		s.report(openAt, "delimiter opens and closes in different conditional regions")
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
// resolved it, so the body may hold lines the scanner never sees, a line
// closing the block among them. A block read raw holds no such line, see
// readsRaw.
func (s *scanner) extensible(raw bool, from int) bool {
	if raw {
		return false
	}
	for at := from; at < s.at; at++ {
		if classify(s.src, s.lines[at]).kind == shapeDirective {
			return true
		}
	}
	return false
}

// skipVerbatim advances to the line closing a verbatim block, tracking the
// conditional regions a directive inside it opens, see readsRaw.
func (s *scanner) skipVerbatim(closer []byte, raw bool) {
	for !s.done() && !bytes.Equal(s.text(s.lines[s.at]), closer) {
		if inner := s.shape(); inner.kind == shapeDirective && !raw {
			s.track(inner)
		}
		s.at++
	}
}

// closesInside finds a line of the body equal to the one that opened the block.
// Asciidoctor ends a block on the first such line, however deep the nodes read
// so far put it, so the tree holds blocks the document does not have. Which
// reading was meant is not the scanner's to guess.
//
// A verbatim block and a table cannot hold one, since skipVerbatim stops there.
func (s *scanner) closesInside(closer []byte, from int) (int, bool) {
	for at := from; at < s.at; at++ {
		if bytes.Equal(s.text(s.lines[at]), closer) {
			return at, true
		}
	}
	return 0, false
}

func (s *scanner) closingLine() Span {
	if s.done() {
		return Span{}
	}
	return s.take()
}

// frontMatter reads the YAML block a static site generator puts first. It is
// detected on the raw input because AsciiDoc would read it as a thematic break
// plus prose and reformat it.
//
// Asciidoctor itself does not skip it without skip-front-matter: it reads the
// opening fence as a thematic break and everything under it up to the first
// blank line as one paragraph. That is why the block freezes what follows.
func (s *scanner) frontMatter() Node {
	start := s.pos()
	for !s.done() && s.shape().kind == shapeBlank {
		s.at++
	}
	if s.done() || !s.onFence() {
		s.at = 0
		return nil
	}
	if s.pos() > start {
		// The fence opens front matter on the first line only, so the blank
		// lines above it are what keeps this document from having any. They
		// stay whether or not a closing fence follows, since a rule may move
		// that line and the next run would decide otherwise.
		s.at, s.freeze = 0, true
		return nil
	}
	s.at++
	for !s.done() && !s.onFence() {
		s.at++
	}
	if s.done() {
		s.at = 0
		return nil
	}
	s.at++
	s.freeze = true
	b := base{gap: Gap{Span: Span{start, start}}, lines: Span{start, s.pos()}}
	return &FrontMatter{base: b}
}

func (s *scanner) onFence() bool {
	return bytes.Equal(s.text(s.lines[s.at]), frontMatterFence)
}
