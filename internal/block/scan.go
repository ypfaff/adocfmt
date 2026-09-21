package block

import (
	"bytes"
	"errors"
	"fmt"
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
	if err := checkLineEndings(src[start:]); err != nil {
		return nil, err
	}

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
	doc.Findings = s.findings
	return doc, nil
}

func checkLineEndings(src []byte) error {
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
				return errors.New("source has carriage returns that end no line")
			}
		}
	}
	if crlf > 0 && lf > 0 {
		return fmt.Errorf("source has mixed line endings: %d CRLF, %d LF", crlf, lf)
	}
	return nil
}

type scanner struct {
	src   []byte
	lines []line
	at    int

	// atStart is true until the first block that cannot precede the document
	// header, so a level 0 title further down stays an ordinary section.
	atStart bool
	// freeze carries a directive's reach to the gap that follows it.
	freeze bool
	// markers are the list markers of the open lists, innermost last.
	markers []string
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
	s.freeze = false
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
	b := base{Gap: gap}
	for s.metaLine(&b, closer) {
		b.Gap = s.gap()
	}
	return s.block(b, closer, false)
}

// metaLine reads the metadata line at the current position into b and reports
// whether there was one. A directive counts once metadata lines stand above
// it, see MetaDirective.
func (s *scanner) metaLine(b *base, closer []byte) bool {
	if s.done() || s.closes(closer) {
		return false
	}
	sh := s.shape()
	kind, ok := metaKind(sh.kind)
	if !ok && (sh.kind != shapeDirective || len(b.Meta) == 0) {
		return false
	}
	if sh.kind == shapeDirective {
		s.track(sh)
		kind, b.Frozen = MetaDirective, true
		b.Gap.Frozen, s.freeze = true, true
	}
	b.Meta = append(b.Meta, Meta{Kind: kind, Gap: b.Gap, Lines: s.take()})
	return true
}

// block is the node the metadata lines in b bind to, or an Opaque one holding
// only them when no block follows: where the caller says the item being read
// ends, at the end of the source or the enclosing block, or at an item of an
// open list, which ends the item whatever came before it.
func (s *scanner) block(b base, closer []byte, orphan bool) Node {
	b.Lines = Span{s.pos(), s.pos()}
	var node Node
	if orphan || s.done() || s.closes(closer) || (len(b.Meta) > 0 && s.sibling()) {
		node = &Opaque{base: b}
	} else {
		node = s.body(b, closer)
	}
	if b.Frozen {
		s.freeze = true
	}
	return node
}

func metaKind(kind shapeKind) (MetaKind, bool) {
	switch kind {
	case shapeAttributes:
		return MetaAttributes, true
	case shapeTitle:
		return MetaTitle, true
	case shapeAnchor:
		return MetaAnchor, true
	case shapeComment:
		return MetaComment, true
	default:
		return 0, false
	}
}

// body reads the block itself. Section titles exist at section level only:
// inside a delimited block or a list item, Asciidoctor reads a title line as
// prose and a would-be underline as the delimiter it looks like. Only a
// discrete style makes a heading out of the line anywhere.
//
// A level 0 title in either form is the document header while nothing but
// attribute entries and directives came before it; Asciidoctor allows both
// above the title.
func (s *scanner) body(b base, closer []byte) Node {
	sh := s.shape()
	atStart := s.atStart
	s.atStart = atStart && (sh.kind == shapeAttrEntry || sh.kind == shapeDirective)

	sectionLevel := closer == nil && len(s.markers) == 0
	if sectionLevel && startsTitle(sh.kind) {
		if level, ok := s.setextLevel(); ok {
			if atStart && level == 0 {
				return s.header(b)
			}
			return s.setext(b, level)
		}
	}
	switch sh.kind {
	case shapeDelimiter:
		return s.delimited(b, sh)
	case shapeHeading:
		if atStart && sh.level == 0 {
			return s.header(b)
		}
		if !sectionLevel && !styledDiscrete(s.src, b.Meta) {
			return s.paragraph(b, closer)
		}
		b.Lines = s.take()
		return &Heading{base: b, Marker: sh.char, Level: sh.level, Title: sh.span}
	case shapeAttrEntry:
		return s.attribute(b)
	case shapeDirective:
		return s.directive(b, sh)
	case shapeMarker:
		return s.list(b, sh, closer)
	case shapeContinuation:
		b.Gap.Frozen = true
		b.Lines = s.take()
		return &Continuation{base: b}
	case shapeIndented:
		return s.literal(b, closer)
	case shapeBreak, shapeMacro:
		b.Lines = s.take()
		return &Opaque{base: b}
	case shapeQuote:
		s.textRun(&b, closer, s.inList())
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

// header is the document header, which stays compact: it runs to the first
// blank line, author, revision and attribute lines included.
func (s *scanner) header(b base) Node {
	start := s.pos()
	for !s.done() && s.shape().kind != shapeBlank {
		if s.shape().kind == shapeAttrEntry {
			s.entry(&b)
			continue
		}
		s.at++
	}
	b.Lines = Span{start, s.pos()}
	return &Header{base: b}
}

func (s *scanner) attribute(b base) Node {
	start := s.pos()
	s.entry(&b)
	b.Lines = Span{start, s.pos()}
	return &Attribute{base: b}
}

// entry reads the attribute entry on the current line with the lines that
// continue its value. Asciidoctor takes every line up to a blank one for as
// long as the line before ends in the entry's own wrap marker, whatever the
// line looks like. A directive among them it resolves first, so the entry
// freezes the way a paragraph around a directive does.
func (s *scanner) entry(b *base) {
	e, _ := parseAttrEntry(s.text(s.lines[s.at]))
	s.bind(e)
	s.at++
	for open := e.wrap != ""; open && !s.done(); s.at++ {
		sh := s.shape()
		if sh.kind == shapeBlank {
			return
		}
		if sh.kind == shapeDirective {
			s.track(sh)
			b.Frozen, b.Gap.Frozen, s.freeze = true, true, true
			continue
		}
		open = bytes.HasSuffix(bytes.TrimLeft(s.text(s.lines[s.at]), " \t"), []byte(e.wrap))
	}
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
	b.Lines = Span{start, s.pos()}
	return &Setext{base: b, Level: level, Title: title}
}

// paragraph reads prose, unless its line breaks carry meaning: a verbatim
// style, the hardbreaks option, or a document attribute in force that binds
// lines makes it a Literal. A quoted paragraph is Opaque like the Markdown
// quote, since Asciidoctor lifts its last line out as the attribution.
func (s *scanner) paragraph(b base, closer []byte) Node {
	first := s.at
	s.textRun(&b, closer, s.inList())
	switch {
	case !paragraphStyles[styleOf(s.src, b.Meta)] && quotedParagraph(s.src, s.lines[first:s.at]):
		return &Opaque{base: b}
	case styledVerbatim(s.src, b.Meta) || hasOption(s.src, b.Meta, hardbreaksOption) || len(s.lineBound) > 0:
		return &Literal{base: b}
	default:
		return &Paragraph{base: b}
	}
}

// literal is an indented paragraph, which AsciiDoc reads as verbatim content.
func (s *scanner) literal(b base, closer []byte) Node {
	s.textRun(&b, closer, false)
	return &Literal{base: b}
}

// textRun consumes the lines of an undelimited block. A directive line inside
// one does not end it: what the directive pulls in decides where the block
// really ends, so the block and the gaps around it freeze instead. A comment
// line does not end it either, nor does a line that has to stay a line of its
// own; the block freezes to keep them where they are.
//
// Inside a list the run of a paragraph also ends at the next item, which is
// what keeps a term from being read as the text of the item above it. A
// literal paragraph does not: Asciidoctor reads it up to a blank line whatever
// its lines look like, an item line included.
func (s *scanner) textRun(b *base, closer []byte, endsAtItem bool) {
	start := s.pos()
	directive := false
	for !s.done() && !s.closes(closer) {
		sh := s.shape()
		if endsText(sh.kind) || (endsAtItem && sh.kind == shapeMarker) {
			break
		}
		if sh.kind == shapeDirective {
			s.track(sh)
			directive = true
		}
		b.Frozen = b.Frozen || sh.kind == shapeDirective || sh.kind == shapeComment || pinsLine(s.text(s.lines[s.at]))
		s.at++
	}
	b.Lines = Span{start, s.pos()}
	if directive {
		b.Gap.Frozen = true
		s.freeze = true
	}
}

// endsText reports whether a line of this shape ends the text of a paragraph
// or a list item. Asciidoctor stops at a blank line, a delimiter, a block
// attribute or anchor line and a lone +, and reads on past a heading line or a
// comment.
func endsText(kind shapeKind) bool {
	switch kind {
	case shapeBlank, shapeDelimiter, shapeAttributes, shapeAnchor, shapeContinuation:
		return true
	default:
		return false
	}
}

func (s *scanner) directive(b base, sh shape) Node {
	s.track(sh)
	b.Gap.Frozen = true
	b.Frozen = true
	b.Lines = s.take()
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
	delim := Delimiter{Char: sh.char, Width: sh.width, Open: open.full}
	style := styleOf(s.src, b.Meta)
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
		delim.Close = s.closingLine()
		b.Lines = Span{start, s.pos()}
		node = &Container{base: b, Delim: delim, Children: children, Tail: tail}
	} else {
		// Asciidoctor reads a comment block without preprocessing it, so a
		// directive inside one opens no conditional region.
		comment := sh.char == '/' || style == commentStyle
		for !s.done() && !bytes.Equal(s.text(s.lines[s.at]), closer) {
			if inner := s.shape(); inner.kind == shapeDirective && !comment {
				s.track(inner)
			}
			s.at++
		}
		delim.Close = s.closingLine()
		b.Lines = Span{start, s.pos()}
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

func (s *scanner) closingLine() Span {
	if s.done() {
		return Span{}
	}
	return s.take()
}

// frontMatter reads the YAML block a static site generator puts first. It is
// detected on the raw input because AsciiDoc would read it as a thematic break
// plus prose and reformat it.
func (s *scanner) frontMatter() Node {
	if s.done() || !bytes.Equal(s.text(s.lines[0]), frontMatterFence) {
		return nil
	}
	start := s.pos()
	s.at++
	for !s.done() && !bytes.Equal(s.text(s.lines[s.at]), frontMatterFence) {
		s.at++
	}
	if s.done() {
		s.at = 0
		return nil
	}
	s.at++
	b := base{Gap: Gap{Span: Span{start, start}}, Lines: Span{start, s.pos()}}
	return &FrontMatter{base: b}
}
