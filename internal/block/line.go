package block

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// line is one source line. full holds every byte, text stops before the
// terminator and any trailing whitespace, because Asciidoctor strips both from
// a line before reading it: a delimiter or directive followed by a space is
// still one.
type line struct {
	text Span
	full Span
}

func splitLines(src []byte, from int) []line {
	var lines []line
	for at := from; at < len(src); {
		end, full := len(src), len(src)
		if next := bytes.IndexByte(src[at:], '\n'); next >= 0 {
			end = at + next
			full = end + 1
		}
		lines = append(lines, line{Span{at, textEnd(src, at, end)}, Span{at, full}})
		at = full
	}
	return lines
}

func textEnd(src []byte, start, end int) int {
	if end > start && src[end-1] == '\r' {
		end--
	}
	for end > start && isSpaceByte(src[end-1]) {
		end--
	}
	return end
}

type shapeKind int

// The line shapes the scanner tells apart. This is the one place that answers
// what a line is; everything downstream reads the answer instead of matching
// again.
const (
	shapeText shapeKind = iota
	shapeBlank
	shapeHeading
	shapeDelimiter
	shapeAttributes
	shapeTitle
	shapeAnchor
	shapeComment
	shapeAttrEntry
	shapeDirective
	shapeMarker
	shapeContinuation
	shapeIndented
	shapeBreak
	shapeMacro
	shapeQuote
)

// content is what a delimited block holds.
type content int

const (
	contentVerbatim content = iota
	contentCompound
	contentTable
)

type shape struct {
	kind    shapeKind
	char    byte
	width   int
	content content
	level   int
	span    Span   // heading title, or list marker
	marker  string // list marker key; items sharing it share a list
	cond    int    // +1 opens a conditional region, -1 closes one
}

// delimiters maps the four-char tip of a delimited block to what it holds,
// mirroring Asciidoctor's DELIMITED_BLOCKS table.
var delimiters = map[byte]content{
	'-': contentVerbatim, // listing
	'.': contentVerbatim, // literal
	'+': contentVerbatim, // passthrough
	'/': contentVerbatim, // comment
	'=': contentCompound, // example
	'*': contentCompound, // sidebar
	'_': contentCompound, // quote
}

var markdownQuote = []byte("> ")

var tocMacro = []byte("toc::")

var blockMacros = [][]byte{[]byte("image::"), []byte("video::"), []byte("audio::"), tocMacro}

var conditionals = [][]byte{[]byte("ifdef::"), []byte("ifndef::"), []byte("ifeval::")}

func classify(src []byte, l line) shape {
	s := src[l.text.Start:l.text.End]
	body := bytes.TrimLeft(s, " \t")
	if len(body) == 0 {
		return shape{kind: shapeBlank}
	}
	indent := l.text.Start + len(s) - len(body)

	if indent > l.text.Start {
		// Markdown allows a thematic break to sit up to three spaces in.
		if spaces := indent - l.text.Start; spaces <= 3 && uniform(s[:spaces], ' ') && isBreak(body) {
			return shape{kind: shapeBreak}
		}
		if sh, ok := markerShape(body, indent); ok {
			return sh
		}
		return shape{kind: shapeIndented}
	}
	if sh, ok := delimiterShape(s); ok {
		return sh
	}
	if isBreak(s) {
		return shape{kind: shapeBreak}
	}
	if sh, ok := headingShape(s, l.text.Start); ok {
		return sh
	}
	if sh, ok := directiveShape(s); ok {
		return sh
	}
	if isBlockMacro(s) {
		return shape{kind: shapeMacro}
	}
	if sh, ok := attrEntryShape(s); ok {
		return sh
	}
	switch {
	case len(s) == 1 && s[0] == '+':
		return shape{kind: shapeContinuation}
	case bytes.HasPrefix(s, []byte("//")):
		return shape{kind: shapeComment}
	case bytes.HasPrefix(s, []byte("[[")) && bytes.HasSuffix(s, []byte("]]")):
		return shape{kind: shapeAnchor}
	case s[0] == '[' && s[len(s)-1] == ']':
		return shape{kind: shapeAttributes}
	case s[0] == '.' && len(s) > 1 && s[1] != ' ' && s[1] != '\t' && s[1] != '.':
		return shape{kind: shapeTitle}
	}
	if bytes.HasPrefix(s, markdownQuote) {
		return shape{kind: shapeQuote}
	}
	if sh, ok := markerShape(s, l.text.Start); ok {
		return sh
	}
	return shape{kind: shapeText}
}

// delimiterShape recognizes a fence. Open blocks are exactly two dashes and
// fenced code exactly three backticks, which a language may follow; everything
// else needs four or more of a char, so the three-char forms in between are
// plain text.
func delimiterShape(s []byte) (shape, bool) {
	switch {
	case len(s) == 2 && s[0] == '-' && s[1] == '-':
		return shape{kind: shapeDelimiter, char: '-', width: 2, content: contentCompound}, true
	case len(s) >= 3 && uniform(s[:3], '`') && (len(s) == 3 || s[3] != '`'):
		return shape{kind: shapeDelimiter, char: '`', width: 3, content: contentVerbatim}, true
	case len(s) >= 4 && isTableChar(s[0]) && uniform(s[1:], '='):
		return shape{kind: shapeDelimiter, char: s[0], width: len(s), content: contentTable}, true
	case len(s) >= 4 && uniform(s, s[0]):
		if held, ok := delimiters[s[0]]; ok {
			return shape{kind: shapeDelimiter, char: s[0], width: len(s), content: held}, true
		}
	}
	return shape{}, false
}

func isTableChar(c byte) bool {
	return c == '|' || c == ',' || c == ':' || c == '!'
}

// headingShape recognizes a one-line section title. Asciidoctor reads the
// Markdown marker as one too, and treats a repeated marker at the end of the
// line as decoration rather than title text.
func headingShape(s []byte, start int) (shape, bool) {
	if len(s) == 0 || (s[0] != '=' && s[0] != '#') {
		return shape{}, false
	}
	level := 0
	for level < len(s) && s[level] == s[0] {
		level++
	}
	if level > 6 || level == len(s) {
		return shape{}, false
	}

	at := level
	for at < len(s) && (s[at] == ' ' || s[at] == '\t') {
		at++
	}
	if at == level || at == len(s) {
		return shape{}, false
	}
	end := trimTrailingMarker(s[at:], s[:level]) + at
	return shape{kind: shapeHeading, char: s[0], level: level - 1, span: Span{start + at, start + end}}, true
}

// trimTrailingMarker returns the length of the title without the closing
// marker of the == Title == form.
func trimTrailingMarker(title, marker []byte) int {
	rest, ok := bytes.CutSuffix(title, marker)
	if !ok {
		return len(title)
	}
	trimmed := bytes.TrimRight(rest, " \t")
	if len(trimmed) == len(rest) || len(trimmed) == 0 {
		return len(title)
	}
	return len(trimmed)
}

// isBreak recognizes a thematic or page break: three or more apostrophes or
// angle brackets, or exactly three of - * _ spaced evenly.
func isBreak(s []byte) bool {
	if len(s) < 3 {
		return false
	}
	if (s[0] == '\'' || s[0] == '<') && uniform(s, s[0]) {
		return true
	}
	if s[0] != '-' && s[0] != '*' && s[0] != '_' {
		return false
	}
	gap := 0
	for 1+gap < len(s) && s[1+gap] == ' ' {
		gap++
	}
	if len(s) != 3+2*gap || s[1+gap] != s[0] || s[len(s)-1] != s[0] {
		return false
	}
	return gap == 0 || uniform(s[2+gap:2+2*gap], ' ')
}

// isBlockMacro recognizes the block macros Asciidoctor knows without an
// extension. Any other name::target[] is prose to it, and so to us.
func isBlockMacro(s []byte) bool {
	if len(s) == 0 || s[len(s)-1] != ']' {
		return false
	}
	for _, name := range blockMacros {
		rest, ok := bytes.CutPrefix(s, name)
		if !ok {
			continue
		}
		open := bytes.IndexByte(rest, '[')
		if open < 0 {
			return false
		}
		target := rest[:open]
		if bytes.Equal(name, tocMacro) {
			return len(target) == 0
		}
		return len(target) > 0 && !isSpaceByte(target[0]) && !isSpaceByte(target[len(target)-1])
	}
	return false
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' }

func directiveShape(s []byte) (shape, bool) {
	if !bytes.HasSuffix(s, []byte("]")) {
		return shape{}, false
	}
	for _, name := range conditionals {
		if bytes.HasPrefix(s, name) {
			return shape{kind: shapeDirective, cond: 1}, true
		}
	}
	switch {
	case bytes.HasPrefix(s, []byte("endif::")):
		return shape{kind: shapeDirective, cond: -1}, true
	case bytes.HasPrefix(s, []byte("include::")):
		return shape{kind: shapeDirective}, true
	}
	return shape{}, false
}

// attrEntryShape recognizes :name: and :name: value, including the :!name:
// and :name!: unset forms.
func attrEntryShape(s []byte) (shape, bool) {
	if len(s) < 2 || s[0] != ':' {
		return shape{}, false
	}
	at := 1
	if s[at] == '!' {
		at++
	}
	name := at
	for at < len(s) && (isWordByte(s[at]) || s[at] == '-') {
		at++
	}
	if at == name {
		return shape{}, false
	}
	if at < len(s) && s[at] == '!' {
		at++
	}
	if at >= len(s) || s[at] != ':' {
		return shape{}, false
	}
	if at+1 < len(s) && s[at+1] != ' ' && s[at+1] != '\t' {
		return shape{}, false
	}
	return shape{kind: shapeAttrEntry}, true
}

const commentStyle = "comment"

// discreteStyles make a heading line a heading below section level, where it
// is otherwise prose.
var discreteStyles = map[string]bool{"discrete": true, "float": true}

func styledDiscrete(src []byte, meta []Meta) bool {
	return discreteStyles[styleOf(src, meta)]
}

// verbatimStyles are the block styles that turn prose into content no rule may
// reflow. An attribute line above a paragraph is enough to switch it.
var verbatimStyles = map[string]bool{
	"source": true, "listing": true, "literal": true, "verse": true,
	"pass": true, "stem": true, "latexmath": true, "asciimath": true,
}

func styledVerbatim(src []byte, meta []Meta) bool {
	style := styleOf(src, meta)
	return verbatimStyles[style] || style == commentStyle
}

// styleOf is the block style an attribute line assigns. It overrides what the
// delimiter says: [source] on an open block makes it code, [comment] makes it a
// comment block.
func styleOf(src []byte, meta []Meta) string {
	style := ""
	for _, m := range meta {
		if m.Kind == MetaAttributes {
			if named := blockStyle(src[m.Lines.Start:m.Lines.End]); named != "" {
				style = named
			}
		}
	}
	return style
}

// blockStyle reads the style out of an attribute line, which is the first
// positional value: [source,go] is source, [#id] is nothing.
func blockStyle(s []byte) string {
	s = bytes.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' {
		return ""
	}
	s = s[1:]
	if end := bytes.IndexAny(s, ",%#.]"); end >= 0 {
		s = s[:end]
	}
	return string(bytes.ToLower(bytes.TrimSpace(s)))
}

// markerShape recognizes the start of a list item. The marker text is the key
// that decides which items share a list: Asciidoctor nests on any change of
// marker, not on indentation.
func markerShape(s []byte, start int) (shape, bool) {
	width, key, ok := markerOf(s)
	if !ok {
		return shape{}, false
	}
	return shape{kind: shapeMarker, span: Span{start, start + width}, marker: key}, true
}

func markerOf(s []byte) (int, string, bool) {
	switch s[0] {
	case '*', '.':
		run := 0
		for run < len(s) && s[run] == s[0] {
			run++
		}
		if followsSpace(s, run) {
			return run, string(s[:run]), true
		}
	case '-':
		if followsSpace(s, 1) {
			return 1, "-", true
		}
	case '<':
		if end := bytes.IndexByte(s, '>'); end > 1 && isCalloutNumber(s[1:end]) && followsSpace(s, end+1) {
			return end + 1, "<>", true
		}
	}
	if width, key, ok := numberedMarker(s); ok {
		return width, key, true
	}
	return descriptionMarker(s)
}

func numberedMarker(s []byte) (int, string, bool) {
	at := 0
	for at < len(s) && s[at] >= '0' && s[at] <= '9' {
		at++
	}
	if at == 0 || at >= len(s) || s[at] != '.' || !followsSpace(s, at+1) {
		return 0, "", false
	}
	return at + 1, "#.", true
}

// descriptionMarker finds the term separator of a description list. The term
// itself is free text, so the separator is what identifies the list.
func descriptionMarker(s []byte) (int, string, bool) {
	for at := 1; at < len(s); at++ {
		if s[at] != ':' && s[at] != ';' {
			continue
		}
		run := at
		for run < len(s) && s[run] == s[at] {
			run++
		}
		width := run - at
		if s[at] == ';' && width != 2 {
			continue
		}
		if s[at] == ':' && (width < 2 || width > 4) {
			continue
		}
		if followsSpace(s, run) {
			return run, string(s[at:run]), true
		}
	}
	return 0, "", false
}

// followsSpace reports whether a marker of the given width is followed by
// space, which is what separates a list item from prose that starts the same.
func followsSpace(s []byte, at int) bool {
	return at == len(s) || s[at] == ' ' || s[at] == '\t'
}

func isCalloutNumber(s []byte) bool {
	if len(s) == 1 && s[0] == '.' {
		return true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isWordByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func uniform(s []byte, c byte) bool {
	for _, got := range s {
		if got != c {
			return false
		}
	}
	return len(s) > 0
}

// setextLevels maps an underline char to the section level it spells.
var setextLevels = map[byte]int{'=': 0, '-': 1, '~': 2, '^': 3, '+': 4}

// setextLevel reports the level of a title and underline pair. Asciidoctor
// allows the underline to differ by one character, not one byte, and asks of
// the title only that it carry a letter or digit and not open with a dot,
// which is why a list marker or a block macro can be one.
func setextLevel(src []byte, title, underline line) (int, bool) {
	under := src[underline.text.Start:underline.text.End]
	if len(under) == 0 {
		return 0, false
	}
	level, ok := setextLevels[under[0]]
	if !ok || !uniform(under, under[0]) {
		return 0, false
	}
	text := src[title.text.Start:title.text.End]
	if len(text) == 0 || text[0] == '.' || !hasAlphanumeric(text) {
		return 0, false
	}
	if diff := utf8.RuneCount(text) - len(under); diff > 1 || diff < -1 {
		return 0, false
	}
	return level, true
}

func hasAlphanumeric(s []byte) bool {
	for _, r := range string(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
