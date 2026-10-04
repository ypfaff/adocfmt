package block

import (
	"bytes"
	"strings"
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
		full := len(src)
		if next := bytes.IndexByte(src[at:], '\n'); next >= 0 {
			full = at + next + 1
		}
		span := Span{at, full}
		lines = append(lines, line{Span{at, textEnd(src, at, terminator(src, span))}, span})
		at = full
	}
	return lines
}

// rubySpace is what Ruby's String#strip strips: not only spaces and tabs, but
// also \v, \f, \r and NUL. A line holds no \n.
const rubySpace = " \t\v\f\r\x00"

// textEnd strips the rubySpace at the end of a line, as Asciidoctor does to
// every line.
func textEnd(src []byte, start, end int) int {
	for end > start && strings.IndexByte(rubySpace, src[end-1]) >= 0 {
		end--
	}
	return end
}

// terminator reports where the line's terminator begins: at the LF, or at the
// CR of a CRLF. A last line that ends without one has none.
func terminator(src []byte, full Span) int {
	end := full.End
	if end > full.Start && src[end-1] == '\n' {
		end--
		if end > full.Start && src[end-1] == '\r' {
			end--
		}
	}
	return end
}

// TrimTrailing returns src without the whitespace at the end of its lines, see
// textEnd. The terminators stay as they are, so a CRLF source keeps its CRLF and
// a last line without one gains none.
//
// It lives beside the scanner because both ask the same question: the scanner
// cuts a line's content here before it classifies the line, and where a line's
// content ends is one fact with one place.
func TrimTrailing(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for _, l := range splitLines(src, 0) {
		out = append(out, src[l.text.Start:l.text.End]...)
		out = append(out, src[terminator(src, l.full):l.full.End]...)
	}
	return out
}

// Lines returns the content of each line in src, its terminator and its
// trailing whitespace already off. A rule asking what a line holds has to ask
// it the way the scanner does, because that is what decides whether the line
// closes a block.
func Lines(src []byte) [][]byte {
	split := splitLines(src, 0)
	lines := make([][]byte, 0, len(split))
	for _, l := range split {
		lines = append(lines, src[l.text.Start:l.text.End])
	}
	return lines
}

// HeadingTitle reports the title a one-line section title carries, read the way
// the scanner reads it, and whether the line is one at all.
func HeadingTitle(line []byte) ([]byte, bool) {
	sh, ok := headingShape(line, 0)
	if !ok {
		return nil, false
	}
	return line[sh.span.Start:sh.span.End], true
}

// ListMarker returns the marker a line opens a list item with, read the way the
// scanner reads it, and whether it opens one at all. The marker is the key the
// items of a list are compared on rather than the text they carry, see
// List.Marker.
func ListMarker(line []byte) (string, bool) {
	body := bytes.TrimLeft(line, " \t")
	if len(body) == 0 {
		return "", false
	}
	_, key, ok := markerOf(body)
	// An indented callout is no item, as in classify.
	return key, ok && (key != calloutMarker || len(body) == len(line))
}

// LineBelow returns the content of the line under the one at points into, its
// terminator and trailing whitespace off, and nothing where no line follows.
func LineBelow(src []byte, at int) []byte {
	next := bytes.IndexByte(src[at:], '\n')
	if next < 0 {
		return nil
	}
	start := at + next + 1
	end := len(src)
	if last := bytes.IndexByte(src[start:], '\n'); last >= 0 {
		end = start + last + 1
	}
	return src[start:textEnd(src, start, terminator(src, Span{start, end}))]
}

// UnderlinesTitle reports whether the two lines form a two-line section title,
// read the way the scanner reads the pair. The underline has to match the title
// within one character, so a rule that shortens a line asks first: a line it
// makes shorter can reach that length and become a section title.
func UnderlinesTitle(title, underline []byte) bool {
	_, ok := setextPair(title, underline)
	return ok
}

// Terminator returns the line ending src ends with: LF, CRLF, or nothing where
// src ends without one.
func Terminator(src []byte) []byte {
	return src[terminator(src, Span{0, len(src)}):]
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
	fill    byte // the char filling out a fence behind its first one
	width   int
	content content
	level   int
	span    Span   // heading title, or list marker
	marker  string // list marker key; items sharing it share a list
	cond    int    // +1 opens a conditional region, -1 closes one
	bad     bool   // a directive Asciidoctor rejects as malformed
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

var mediaMacros = [][]byte{[]byte("image::"), []byte("video::"), []byte("audio::")}

func classify(src []byte, l line) shape {
	s := src[l.text.Start:l.text.End]
	body := bytes.TrimLeft(s, " \t")
	if len(body) == 0 {
		return shape{kind: shapeBlank}
	}
	indent := l.text.Start + len(s) - len(body)

	if indent > l.text.Start {
		// Markdown allows a thematic break to sit up to three spaces in.
		if spaces := indent - l.text.Start; spaces <= 3 && uniform(s[:spaces], ' ') && isMarkdownBreak(body) {
			return shape{kind: shapeBreak}
		}
		// Asciidoctor reads a callout only at the start of a line.
		if sh, ok := markerShape(body, indent); ok && sh.marker != calloutMarker {
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
	// A third slash makes the line no comment to Asciidoctor.
	case bytes.HasPrefix(s, []byte("//")) && !bytes.HasPrefix(s, []byte("///")):
		return shape{kind: shapeComment}
	case isBlockAnchor(s):
		return shape{kind: shapeAnchor}
	case isBlockAttributeList(s):
		return shape{kind: shapeAttributes}
	case isBlockTitle(s):
		return shape{kind: shapeTitle}
	}
	// Asciidoctor tries a list item before a quote, so > term:: opens a list.
	if sh, ok := markerShape(s, l.text.Start); ok {
		return sh
	}
	if bytes.HasPrefix(s, markdownQuote) {
		return shape{kind: shapeQuote}
	}
	return shape{kind: shapeText}
}

// openBlockWidth is the one width an open block has, and so what tells it apart
// from every other fence.
const openBlockWidth = 2

// delimiterShape recognizes a fence. Open blocks are exactly two dashes and
// fenced code exactly three backticks, which a language may follow; everything
// else needs four or more of a char, so the three-char forms in between are
// plain text.
func delimiterShape(s []byte) (shape, bool) {
	switch {
	case len(s) == openBlockWidth && s[0] == '-' && s[1] == '-':
		return shape{kind: shapeDelimiter, char: '-', fill: '-', width: openBlockWidth, content: contentCompound}, true
	case len(s) >= 3 && uniform(s[:3], '`') && (len(s) == 3 || s[3] != '`'):
		return shape{kind: shapeDelimiter, char: '`', fill: '`', width: 3, content: contentVerbatim}, true
	case len(s) >= 4 && isTableChar(s[0]) && uniform(s[1:], '='):
		return shape{kind: shapeDelimiter, char: s[0], fill: '=', width: len(s), content: contentTable}, true
	case len(s) >= 4 && uniform(s, s[0]):
		if held, ok := delimiters[s[0]]; ok {
			return shape{kind: shapeDelimiter, char: s[0], fill: s[0], width: len(s), content: held}, true
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
// angle brackets, or a Markdown break.
func isBreak(s []byte) bool {
	return len(s) >= 3 && (s[0] == '\'' || s[0] == '<') && uniform(s, s[0]) || isMarkdownBreak(s)
}

// isMarkdownBreak recognizes exactly three of - * _ spaced evenly.
func isMarkdownBreak(s []byte) bool {
	if len(s) < 3 || s[0] != '-' && s[0] != '*' && s[0] != '_' {
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
// extension. Any other name::target[] is prose to it unless an extension
// registers the name, see isCustomBlockMacro.
func isBlockMacro(s []byte) bool {
	if rest, ok := bytes.CutPrefix(s, tocMacro); ok {
		return len(rest) >= 2 && rest[0] == '[' && rest[len(rest)-1] == ']'
	}
	for _, name := range mediaMacros {
		if rest, ok := bytes.CutPrefix(s, name); ok {
			return isMacroTarget(rest, 1)
		}
	}
	return false
}

// isBlockAnchor mirrors Asciidoctor's BlockAnchorRx: an id that starts with a
// letter, _ or :, and reftext after a comma if any. Any other [[...]] line is
// prose to it, and it does not try one as an attribute list either.
func isBlockAnchor(s []byte) bool {
	inner, ok := bytes.CutPrefix(s, []byte("[["))
	if !ok {
		return false
	}
	if inner, ok = bytes.CutSuffix(inner, []byte("]]")); !ok {
		return false
	}
	id, reftext, hasReftext := bytes.Cut(inner, []byte(","))
	if len(id) == 0 {
		return !hasReftext
	}
	if hasReftext && len(reftext) == 0 {
		return false
	}
	for i, r := range string(id) {
		ok := isWordRune(r) || r == '-' || r == ':' || r == '.'
		if i == 0 {
			ok = isAlphaRune(r) || r == '_' || r == ':'
		}
		if !ok {
			return false
		}
	}
	return true
}

// isBlockAttributeList mirrors Asciidoctor's BlockAttributeListRx: the list is
// empty or opens with a word character or one of .#%{,"' so [<foo>] and
// [ source] are prose.
func isBlockAttributeList(s []byte) bool {
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return false
	}
	if len(s) == 2 {
		return true
	}
	first, _ := utf8.DecodeRune(s[1:])
	return isWordRune(first) || strings.ContainsRune(`.#%{,"'`, first)
}

// isBlockTitle mirrors Asciidoctor's BlockTitleRx: a dot and text that starts
// with neither a space nor a dot, though one more dot may open it, so ..Title
// is the title .Title.
func isBlockTitle(s []byte) bool {
	if len(s) < 2 || s[0] != '.' {
		return false
	}
	at := 1
	if s[at] == '.' {
		at++
	}
	return at < len(s) && !isSpaceByte(s[at]) && s[at] != '.'
}

// isCustomBlockMacro mirrors Asciidoctor's CustomBlockMacroRx, the shape of a
// block macro an extension registers: name::target[attributes], see
// isMacroTarget.
func isCustomBlockMacro(s []byte) bool {
	name, rest, ok := bytes.Cut(s, []byte("::"))
	if !ok || len(name) == 0 {
		return false
	}
	for i, r := range string(name) {
		if !isWordRune(r) && (i == 0 || r != '-') {
			return false
		}
	}
	return isMacroTarget(rest, 0)
}

// isMacroTarget reports whether rest, what follows the name:: of a block macro,
// reads as target[attributes] the way Asciidoctor's macro patterns read it: a
// target of at least shortest bytes that neither starts nor ends with a space.
// The attributes may open at any [, so image::[x][] has the target [x].
func isMacroTarget(rest []byte, shortest int) bool {
	if len(rest) < 2 || rest[len(rest)-1] != ']' {
		return false
	}
	for open := shortest; open < len(rest)-1; open++ {
		if rest[open] != '[' {
			continue
		}
		if target := rest[:open]; len(target) == 0 || (!isSpaceByte(target[0]) && !isSpaceByte(target[len(target)-1])) {
			return true
		}
	}
	return false
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' }

// directiveShape recognizes a preprocessor directive, name::target[text],
// mirroring Asciidoctor's ConditionalDirectiveRx and IncludeDirectiveRx: the
// target of a conditional holds no space, and that of an include neither starts
// nor ends with one. ifdef and ifndef with text in the brackets apply to that
// text alone and open no region; ifeval carries its expression there and always
// opens one. What Asciidoctor rejects as malformed is still a directive line,
// marked bad so the scanner reports it.
func directiveShape(s []byte) (shape, bool) {
	name, rest, ok := bytes.Cut(s, []byte("::"))
	if !ok || len(rest) == 0 || rest[len(rest)-1] != ']' {
		return shape{}, false
	}
	open := bytes.IndexByte(rest, '[')
	if open < 0 {
		return shape{}, false
	}
	target, text := rest[:open], rest[open+1:len(rest)-1]

	sh := shape{kind: shapeDirective}
	if string(name) == "include" {
		if len(target) == 0 || isSpaceByte(target[0]) || isSpaceByte(target[len(target)-1]) {
			return shape{}, false
		}
		return sh, true
	}
	if bytes.ContainsAny(target, " \t") {
		return shape{}, false
	}
	switch string(name) {
	case "ifdef", "ifndef":
		sh.bad = len(target) == 0
		if !sh.bad && len(text) == 0 {
			sh.cond = 1
		}
	case "ifeval":
		sh.bad = len(target) > 0 || !isEvalExpression(bytes.Trim(text, rubySpace))
		if !sh.bad {
			sh.cond = 1
		}
	case "endif":
		sh.bad = len(text) > 0
		if !sh.bad {
			sh.cond = -1
		}
	default:
		return shape{}, false
	}
	return sh, true
}

// isEvalExpression mirrors Asciidoctor's EvalExpressionRx: a comparison with
// something on either side of the operator.
func isEvalExpression(s []byte) bool {
	for i := 1; i < len(s)-1; i++ {
		single := s[i] == '<' || s[i] == '>'
		double := (s[i] == '=' || s[i] == '!') && s[i+1] == '=' && i < len(s)-2
		if single || double {
			return true
		}
	}
	return false
}

// pinsLine reports whether a line inside a paragraph has to stay a line of its
// own. A comment line joined into prose becomes prose; a description term
// joined onto the line above makes a list of both; inside a list, a line that
// starts like an item changes how Asciidoctor reads the lines below it, even
// where it renders the line as text; an escaped directive loses its backslash
// only at the start of a line; and an inline {set:} makes Asciidoctor drop the
// whole line it stands on.
func pinsLine(sh shape, s []byte, inList bool) bool {
	if sh.kind == shapeComment || (sh.kind == shapeMarker && (inList || isTermMarker(sh.marker))) {
		return true
	}
	if len(s) > 0 && s[0] == '\\' {
		if _, ok := directiveShape(s[1:]); ok {
			return true
		}
	}
	return bytes.Contains(s, []byte("{set:"))
}

// attrEntryShape recognizes :name: and :name: value, including the :!name:
// and :name!: unset forms.
func attrEntryShape(s []byte) (shape, bool) {
	if _, ok := parseAttrEntry(s); !ok {
		return shape{}, false
	}
	return shape{kind: shapeAttrEntry}, true
}

// attrEntry is an attribute entry as Asciidoctor stores it: the name
// sanitized, the value trimmed, and set false for the :!name: and :name!:
// forms. wrap is the marker that continues the value on the next line; value
// is the first line's share of it, which is all bind needs.
type attrEntry struct {
	name  string
	value string
	set   bool
	wrap  string
}

// wraps are the line endings that continue an attribute value on the next
// line. Asciidoctor joins with a space either way; only a value that ends in
// " +" before the backslash keeps its line break.
var wraps = []string{" \\", " +"}

// parseAttrEntry mirrors Asciidoctor's AttributeEntryRx: the name starts with
// a word character and then runs to the next colon, spaces and dots included,
// and the value is separated by whitespace or absent, so :name:value is prose
// and :name:: a term.
func parseAttrEntry(s []byte) (attrEntry, bool) {
	if len(s) < 2 || s[0] != ':' {
		return attrEntry{}, false
	}
	at := 1
	unset := s[at] == '!'
	if unset {
		at++
	}
	if first, _ := utf8.DecodeRune(s[at:]); !isWordRune(first) {
		return attrEntry{}, false
	}
	end := bytes.IndexByte(s[at:], ':')
	if end < 0 {
		return attrEntry{}, false
	}
	name := s[at : at+end]
	at += end + 1
	if name[len(name)-1] == '!' {
		unset = true
		name = name[:len(name)-1]
	}
	if at < len(s) && !isSpaceByte(s[at]) {
		return attrEntry{}, false
	}
	e := attrEntry{name: sanitizeAttrName(name), set: !unset}
	value := bytes.TrimSpace(s[at:])
	for _, wrap := range wraps {
		if bytes.HasSuffix(value, []byte(wrap)) {
			e.wrap = wrap
			value = bytes.TrimRight(value[:len(value)-len(wrap)], " \t")
			break
		}
	}
	e.value = string(value)
	return e, true
}

// sanitizeAttrName stores the name the way Asciidoctor does, everything but
// word characters and dashes dropped and the rest lowercased, so :Hard Breaks:
// sets hardbreaks.
func sanitizeAttrName(name []byte) string {
	kept := strings.Map(func(r rune) rune {
		if isWordRune(r) || r == '-' {
			return r
		}
		return -1
	}, string(name))
	return strings.ToLower(kept)
}

var attribution = []byte("-- ")

// quotedParagraph recognizes the paragraph Asciidoctor reads as a quote block
// with an attribution: it opens with a straight double quote, its last line
// starts with two dashes and a space, and the line before that closes the
// quote. Comment lines do not count, since Asciidoctor drops them first.
func quotedParagraph(src []byte, lines []line) bool {
	var texts [][]byte
	for _, l := range lines {
		if classify(src, l).kind != shapeComment {
			texts = append(texts, src[l.text.Start:l.text.End])
		}
	}
	n := len(texts)
	if n < 2 || len(texts[0]) == 0 {
		return false
	}
	return texts[0][0] == '"' && bytes.HasSuffix(texts[n-2], []byte{'"'}) && bytes.HasPrefix(texts[n-1], attribution)
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
			return end + 1, calloutMarker, true
		}
	}
	if bytes.HasPrefix(s, bullet) && followsSpace(s, len(bullet)) {
		return len(bullet), string(bullet), true
	}
	if width, key, ok := numberedMarker(s); ok {
		return width, key, true
	}
	return descriptionMarker(s)
}

// calloutMarker is the key every callout number shares.
const calloutMarker = "<>"

// bullet is the one non-ASCII list marker Asciidoctor knows.
var bullet = []byte("\u2022")

// numberedMarker recognizes an ordered list marker. The key is the marker
// Asciidoctor normalizes the item to, since that is what it compares to decide
// whether the next item continues the list or nests: 1. and 2. share a list,
// a. after 1. does not.
func numberedMarker(s []byte) (int, string, bool) {
	run := 0
	for run < len(s) && isDigit(s[run]) {
		run++
	}
	if run > 0 {
		if run < len(s) && s[run] == '.' && followsSpace(s, run+1) {
			return run + 1, "1.", true
		}
		return 0, "", false
	}
	for run < len(s) && bytes.IndexByte(romanDigits, s[run]) >= 0 {
		run++
	}
	if run > 0 && run < len(s) && s[run] == ')' && followsSpace(s, run+1) {
		// Asciidoctor styles a mixed-case run by the letter before the paren.
		if isUpper(s[run-1]) {
			return run + 1, "I)", true
		}
		return run + 1, "i)", true
	}
	if len(s) > 1 && s[1] == '.' && followsSpace(s, 2) {
		switch {
		case isUpper(s[0]):
			return 2, "A.", true
		case isLower(s[0]):
			return 2, "a.", true
		}
	}
	return 0, "", false
}

var romanDigits = []byte("IVXivx")

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

func isTermMarker(marker string) bool { return marker[0] == ':' || marker[0] == ';' }

// termShape reads the line as a description list term, whatever else it reads
// as. Asciidoctor tests a line that way for the next term of a description
// list and for a description list nested in an item, so * a:: b is a term
// there. A comment line is none.
func termShape(src []byte, l line) (shape, bool) {
	s := src[l.text.Start:l.text.End]
	if len(s) > 2 && s[0] == '/' && s[1] == '/' && s[2] != '/' {
		return shape{}, false
	}
	body := bytes.TrimLeft(s, " \t")
	if len(body) == 0 {
		return shape{}, false
	}
	width, key, ok := descriptionMarker(body)
	if !ok {
		return shape{}, false
	}
	start := l.text.Start + len(s) - len(body)
	return shape{kind: shapeMarker, span: Span{start, start + width}, marker: key}, true
}

// opensAnyList reports whether the line opens an item of any list, which is
// what Asciidoctor's AnyListRx matches.
func opensAnyList(src []byte, l line) bool {
	_, isTerm := termShape(src, l)
	return isTerm || classify(src, l).kind == shapeMarker
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
		if run == len(s) || followsSpace(s, run) {
			return run, string(s[at:run]), true
		}
	}
	return 0, "", false
}

// followsSpace reports whether a marker of the given width is followed by
// space, which is what separates a list item from prose that starts the same.
// The line ends before its trailing whitespace, so a marker alone on its line
// is prose; only a description term may stand alone.
func followsSpace(s []byte, at int) bool {
	return at < len(s) && isSpaceByte(s[at])
}

func isCalloutNumber(s []byte) bool {
	if len(s) == 1 && s[0] == '.' {
		return true
	}
	for _, c := range s {
		if !isDigit(c) {
			return false
		}
	}
	return len(s) > 0
}

// isWordRune is Ruby's \p{Word}, which is what Asciidoctor asks of the first
// character of an attribute name and keeps of the rest.
func isWordRune(r rune) bool {
	return isAlphaRune(r) || unicode.IsMark(r) || unicode.IsDigit(r) || unicode.Is(unicode.Pc, r) ||
		unicode.Is(unicode.Join_Control, r)
}

// isAlphaRune is Ruby's \p{Alpha}, which is what Asciidoctor asks of the first
// character of a block anchor's id.
func isAlphaRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_Alphabetic, r)
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
	return setextPair(src[title.text.Start:title.text.End], src[underline.text.Start:underline.text.End])
}

// setextPair reports the level the two lines spell, and whether they form a
// pair at all.
func setextPair(title, underline []byte) (int, bool) {
	if len(underline) == 0 {
		return 0, false
	}
	level, ok := setextLevels[underline[0]]
	if !ok || !uniform(underline, underline[0]) {
		return 0, false
	}
	if len(title) == 0 || title[0] == '.' || !hasAlphanumeric(title) {
		return 0, false
	}
	if diff := utf8.RuneCount(title) - len(underline); diff > 1 || diff < -1 {
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
