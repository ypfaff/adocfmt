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

	s := &scanner{src: src, lines: splitLines(src, start), atStart: true}
	if matter := s.frontMatter(); matter != nil {
		doc.Nodes = append(doc.Nodes, matter)
	}
	nodes, tail := s.nodes(nil)
	doc.Nodes = append(doc.Nodes, nodes...)
	doc.Tail = tail
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

	// atStart keeps the document header from claiming a level 0 title further
	// down, where it is an ordinary section.
	atStart bool
	// freeze carries a directive's reach to the gap that follows it.
	freeze bool
	// markers are the list markers of the open lists, innermost last.
	markers []string
	// regions are the open conditional regions, innermost last. A delimiter
	// that opens in one and closes in another means two documents in one.
	regions    []int
	lastRegion int

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

func (s *scanner) report(at int, severity Severity, message string) {
	s.findings = append(s.findings, Finding{Line: at + 1, Severity: severity, Message: message})
}

func (s *scanner) track(sh shape) {
	switch {
	case sh.cond > 0:
		s.lastRegion++
		s.regions = append(s.regions, s.lastRegion)
	case sh.cond < 0 && len(s.regions) > 0:
		s.regions = s.regions[:len(s.regions)-1]
	}
}

func (s *scanner) region() int {
	if len(s.regions) == 0 {
		return 0
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

func (s *scanner) node(gap Gap, closer []byte) Node {
	atStart := s.atStart
	s.atStart = false

	var meta []Meta
	frozen := false
	for !s.done() && !s.closes(closer) {
		sh := s.shape()
		kind, ok := metaKind(sh.kind)
		if !ok && (sh.kind != shapeDirective || len(meta) == 0) {
			break
		}
		if sh.kind == shapeDirective {
			s.track(sh)
			kind, frozen = MetaDirective, true
			gap.Frozen, s.freeze = true, true
		}
		meta = append(meta, Meta{Kind: kind, Gap: gap, Lines: s.take()})
		gap = s.gap()
	}

	b := base{Gap: gap, Meta: meta, Lines: Span{s.pos(), s.pos()}, Frozen: frozen}
	var node Node
	if s.done() || s.closes(closer) {
		node = &Opaque{base: b}
	} else {
		node = s.body(b, atStart, closer)
	}
	if frozen {
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

func (s *scanner) body(b base, atStart bool, closer []byte) Node {
	sh := s.shape()
	if startsTitle(sh.kind) {
		if node, ok := s.setext(b, closer); ok {
			return node
		}
	}
	switch sh.kind {
	case shapeDelimiter:
		return s.delimited(b, sh)
	case shapeHeading:
		if atStart && sh.level == 0 {
			return s.header(b)
		}
		b.Lines = s.take()
		return &Heading{base: b, Marker: sh.char, Level: sh.level, Title: sh.span}
	case shapeAttrEntry:
		b.Lines = s.take()
		return &Attribute{base: b}
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
		s.textRun(&b, closer)
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
		s.at++
	}
	b.Lines = Span{start, s.pos()}
	return &Header{base: b}
}

// setext reads a two-line title. The line closing an enclosing block is never
// an underline: Asciidoctor reads a delimited block up to its terminator before
// it parses the content.
func (s *scanner) setext(b base, closer []byte) (Node, bool) {
	if s.at+1 >= len(s.lines) {
		return nil, false
	}
	if closer != nil && bytes.Equal(s.text(s.lines[s.at+1]), closer) {
		return nil, false
	}
	level, ok := setextLevel(s.src, s.lines[s.at], s.lines[s.at+1])
	if !ok {
		return nil, false
	}
	title := s.lines[s.at].text
	start := s.pos()
	s.at += 2
	b.Lines = Span{start, s.pos()}
	return &Setext{base: b, Level: level, Title: title}, true
}

func (s *scanner) paragraph(b base, closer []byte) Node {
	s.textRun(&b, closer)
	if styledVerbatim(s.src, b.Meta) {
		return &Literal{base: b}
	}
	return &Paragraph{base: b}
}

// literal is an indented paragraph, which AsciiDoc reads as verbatim content.
func (s *scanner) literal(b base, closer []byte) Node {
	s.textRun(&b, closer)
	return &Literal{base: b}
}

// textRun consumes the lines of an undelimited block. A directive line inside
// one does not end it: what the directive pulls in decides where the block
// really ends, so the block and the gaps around it freeze instead.
//
// Inside a list the run also ends at the next item, which is what keeps a term
// from being read as the text of the item above it.
func (s *scanner) textRun(b *base, closer []byte) {
	start := s.pos()
	inList := len(s.markers) > 0
	for !s.done() && !s.closes(closer) {
		sh := s.shape()
		if sh.kind == shapeBlank || sh.kind == shapeDelimiter || sh.kind == shapeAttributes {
			break
		}
		if sh.kind == shapeMarker && inList {
			break
		}
		if sh.kind == shapeDirective {
			s.track(sh)
			b.Frozen = true
		}
		s.at++
	}
	b.Lines = Span{start, s.pos()}
	if b.Frozen {
		b.Gap.Frozen = true
		s.freeze = true
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
		children, tail := s.nodes(s.text(open))
		delim.Close = s.closingLine()
		b.Lines = Span{start, s.pos()}
		node = &Container{base: b, Delim: delim, Children: children, Tail: tail}
	} else {
		// Asciidoctor reads a comment block without preprocessing it, so a
		// directive inside one opens no conditional region.
		comment := sh.char == '/' || style == commentStyle
		for !s.done() && !bytes.Equal(s.text(s.lines[s.at]), s.text(open)) {
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

	switch {
	case !delim.Closed():
		s.report(openAt, Warn, "block has no closing delimiter")
	case s.region() != region:
		s.report(openAt, Skip, "delimiter opens and closes in different conditional regions")
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
