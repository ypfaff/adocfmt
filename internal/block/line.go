package block

import (
	"bytes"
	"slices"
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

var blockMacros = [][]byte{[]byte("image::"), []byte("video::"), []byte("audio::"), tocMacro}

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

// directiveShape recognizes a preprocessor directive, name::target[text].
// ifdef and ifndef with text in the brackets apply to that text alone and open
// no region; ifeval carries its expression there and always opens one. What
// Asciidoctor rejects as malformed is still a directive line, marked bad so
// the scanner reports it.
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
	switch string(name) {
	case "ifdef", "ifndef":
		sh.bad = len(target) == 0
		if !sh.bad && len(text) == 0 {
			sh.cond = 1
		}
	case "ifeval":
		sh.bad = len(target) > 0 || len(text) == 0
		if !sh.bad {
			sh.cond = 1
		}
	case "endif":
		sh.cond = -1
	case "include":
		if len(target) == 0 {
			return shape{}, false
		}
	default:
		return shape{}, false
	}
	return sh, true
}

// pinsLine reports whether a line of prose has to stay a line of its own. An
// escaped directive loses its backslash only at the start of a line, and an
// inline {set:} makes Asciidoctor drop the whole line it stands on.
func pinsLine(s []byte) bool {
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
// positional value: [source,go] is source, ["source"] too, [#id] and
// [role=x] are nothing.
func blockStyle(s []byte) string {
	s = bytes.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' {
		return ""
	}
	s = s[1:]
	if end := bytes.IndexAny(s, ",%#.]"); end >= 0 {
		s = s[:end]
	}
	if bytes.IndexByte(s, '=') >= 0 {
		return ""
	}
	s = bytes.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	return string(bytes.ToLower(s))
}

// paragraphStyles hand a paragraph to block parsing, where Asciidoctor never
// looks for the quoted paragraph form: its PARAGRAPH_STYLES but normal, and the
// admonition styles. Any other style it drops as unknown.
var paragraphStyles = map[string]bool{
	"comment": true, "example": true, "literal": true, "listing": true, "open": true,
	"pass": true, "quote": true, "sidebar": true, "source": true, "verse": true,
	"abstract": true, "partintro": true,
	"note": true, "tip": true, "important": true, "warning": true, "caution": true,
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

// hardbreaksOption makes every line break of a paragraph a <br>, so its lines
// are content the way the lines of a verse are.
const hardbreaksOption = "hardbreaks"

// hasOption reports whether an attribute line above the block sets the option.
// Unlike a style, options accumulate across attribute lines.
func hasOption(src []byte, meta []Meta, option string) bool {
	for _, m := range meta {
		if m.Kind == MetaAttributes && slices.Contains(blockOptions(src[m.Lines.Start:m.Lines.End]), option) {
			return true
		}
	}
	return false
}

// blockOptions lists the options an attribute line sets, in every spelling
// Asciidoctor accepts: %opt in the shorthand of the first attribute, options=
// or opts= holding one name or a comma-separated list, and opt-option=, the
// key it stores every one of them under.
func blockOptions(s []byte) []string {
	s = bytes.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil
	}
	var opts []string
	for i, attr := range splitAttrList(s[1 : len(s)-1]) {
		name, value, named := cutAttr(attr)
		switch {
		case !named && i == 0:
			opts = append(opts, shorthandOptions(name)...)
		case named && (name == "options" || name == "opts"):
			opts = append(opts, splitOptions(value)...)
		case named && strings.HasSuffix(name, "-option"):
			opts = append(opts, strings.TrimSuffix(name, "-option"))
		}
	}
	return opts
}

// splitAttrList cuts an attribute list at its commas. A quote shields a comma
// only where Asciidoctor reads one as quoting: at the start of an attribute or
// right after its =, and only when a closing quote follows.
func splitAttrList(s []byte) [][]byte {
	var attrs [][]byte
	start := 0
	for at := 0; at < len(s); at++ {
		switch c := s[at]; {
		case c == ',':
			attrs = append(attrs, s[start:at])
			start = at + 1
		case (c == '"' || c == '\'') && opensQuote(s[start:at]):
			if closing := closingQuote(s, at); closing > 0 {
				at = closing
			}
		}
	}
	return append(attrs, s[start:])
}

func opensQuote(before []byte) bool {
	before = bytes.TrimSpace(before)
	return len(before) == 0 || before[len(before)-1] == '='
}

// closingQuote finds the quote that closes the one at open, skipping escaped
// quotes, or -1 when none does and the opener is text.
func closingQuote(s []byte, open int) int {
	for at := open + 1; at < len(s); at++ {
		switch s[at] {
		case '\\':
			at++
		case s[open]:
			return at
		}
	}
	return -1
}

// cutAttr splits one attribute into name and value. A quoted attribute is
// positional whatever it contains; anything else is named at its first =.
func cutAttr(attr []byte) (name, value string, named bool) {
	attr = bytes.TrimSpace(attr)
	if len(attr) > 0 && (attr[0] == '"' || attr[0] == '\'') {
		return unquote(attr), "", false
	}
	n, v, ok := bytes.Cut(attr, []byte{'='})
	if !ok {
		return string(attr), "", false
	}
	return string(bytes.TrimSpace(n)), unquote(bytes.TrimSpace(v)), true
}

func unquote(s []byte) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	return string(s)
}

// shorthandOptions reads the %opt parts out of the style shorthand
// [style#id.role%opt]. A space anywhere means the attribute is no shorthand.
func shorthandOptions(style string) []string {
	if strings.ContainsRune(style, ' ') {
		return nil
	}
	var opts []string
	for at := 0; at < len(style); {
		end := len(style)
		if next := strings.IndexAny(style[at+1:], ".#%"); next >= 0 {
			end = at + 1 + next
		}
		if style[at] == '%' && end > at+1 {
			opts = append(opts, style[at+1:end])
		}
		at = end
	}
	return opts
}

// splitOptions reads the value of options= the way Asciidoctor does: a list
// loses its spaces before it is split, a single name keeps them and then
// matches nothing.
func splitOptions(value string) []string {
	if strings.Contains(value, ",") {
		value = strings.ReplaceAll(value, " ", "")
	}
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' })
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
	if bytes.HasPrefix(s, bullet) && followsSpace(s, len(bullet)) {
		return len(bullet), string(bullet), true
	}
	if width, key, ok := numberedMarker(s); ok {
		return width, key, true
	}
	return descriptionMarker(s)
}

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
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) || unicode.Is(unicode.Pc, r)
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
