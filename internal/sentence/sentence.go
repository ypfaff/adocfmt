// Package sentence reflows the text of a paragraph to one sentence per line.
//
// It reads inline syntax only as far as the reflow needs it: where a sentence
// ends, and which spans pass their text through and so are never split or
// joined inside.
package sentence

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reflow joins lines into one run of text and splits it again after every
// sentence. A hard line break stays a line end, and so does a line end inside
// an opaque span.
func Reflow(lines [][]byte) [][]byte {
	t := newText(bytes.Join(lines, lineEnd))

	var out []byte
	for i := range t.words {
		out = append(out, t.word(i)...)
		if i < len(t.words)-1 {
			out = append(out, t.between(i)...)
		}
	}
	return bytes.Split(out, lineEnd)
}

var lineEnd = []byte("\n")

// A span is a run of the text by its byte offsets.
type span struct{ start, end int }

// text is a paragraph cut into words, with the opaque spans in it.
type text struct {
	src   []byte
	words []span
	// opaque holds the spans that are never split or joined, in order.
	opaque []span
}

func newText(src []byte) *text {
	t := &text{src: src}
	t.words = t.split()
	return t
}

func (t *text) word(i int) []byte { return t.src[t.words[i].start:t.words[i].end] }

// between returns what the reflow writes between word i and the next.
func (t *text) between(i int) []byte {
	gap := t.src[t.words[i].end:t.words[i+1].start]
	lineBreak := bytes.Contains(gap, lineEnd)
	switch {
	case lineBreak && t.hardBreak(i):
		return gap
	case t.endsSentence(i):
		return lineEnd
	case lineBreak:
		return []byte(" ")
	default:
		return gap
	}
}

// split cuts the text into words at whitespace.
func (t *text) split() []span {
	var words []span
	i := 0
	for {
		for i < len(t.src) && isSpace(t.src[i]) {
			i++
		}
		if i == len(t.src) {
			return words
		}
		start := i
		i = t.wordEnd(i)
		words = append(words, span{start, i})
	}
}

// wordEnd returns where the word starting at i ends, and records the opaque
// spans on the way. A span is never cut, so the word runs on over any
// whitespace inside one.
func (t *text) wordEnd(i int) int {
	for i < len(t.src) && !isSpace(t.src[i]) {
		if end, ok := spanEnd(t.src, i); ok {
			t.opaque = append(t.opaque, span{i, end})
			i = end
		} else {
			i++
		}
	}
	return i
}

// hardBreak reports whether word i, at the end of a line, is the + of a hard
// line break, which Asciidoctor reads only after a space.
func (t *text) hardBreak(i int) bool {
	start := t.words[i].start
	return string(t.word(i)) == "+" && start > 0 && t.src[start-1] == ' '
}

// endsSentence reports whether a sentence ends after word i.
func (t *text) endsSentence(i int) bool {
	return t.endsInMark(t.words[i]) && startsSentence(t.word(i+1))
}

// endsInMark reports whether w ends in a mark that ends a sentence, past any
// closing quotes, brackets and markup. A mark inside an opaque span ends none,
// so neither does the period that ends the text of a footnote or of a backtick
// span.
func (t *text) endsInMark(w span) bool {
	body := bytes.TrimRight(t.src[w.start:w.end], closers)
	if len(body) == 0 || t.inOpaque(w.start+len(body)-1) {
		return false
	}
	switch body[len(body)-1] {
	case '!', '?':
		return true
	case '.':
		return fullStop(body[:len(body)-1])
	default:
		return false
	}
}

func (t *text) inOpaque(i int) bool {
	for _, s := range t.opaque {
		if s.start <= i && i < s.end {
			return true
		}
	}
	return false
}

// startsSentence reports whether word starts with an uppercase letter, past
// any opening quotes, brackets and markup.
func startsSentence(word []byte) bool {
	r, _ := utf8.DecodeRune(bytes.TrimLeft(word, openers))
	return unicode.IsUpper(r)
}

// fullStop reports whether a period after before ends a sentence, rather than
// an ellipsis, an ordinal, an initial or an abbreviation.
func fullStop(before []byte) bool {
	r, _ := utf8.DecodeLastRune(before)
	if r == '.' || unicode.IsDigit(r) {
		return false
	}
	word := before[len(bytes.TrimRightFunc(before, unicode.IsLetter)):]
	return utf8.RuneCount(word) != 1 && !abbreviations[strings.ToLower(string(word))]
}

// German quotes close with “ and ‘, and guillemets point either way, «…» in
// French and »…« in German, so the two sets share them.
const (
	closers = ")]\"'”’“‘»«*_#"
	openers = "([\"'“‘„«»‚*_#`"
)

// abbreviations end in a period that ends no sentence. Only the letters before
// the final period count, so e.g. and z. B. are caught as initials.
var abbreviations = map[string]bool{
	// English
	"approx": true, "cf": true, "ch": true, "dr": true, "eq": true,
	"etc": true, "fig": true, "figs": true, "jr": true, "mr": true,
	"mrs": true, "ms": true, "no": true, "nos": true, "prof": true,
	"sec": true, "sr": true, "st": true, "vol": true, "vs": true,
	// German
	"abb": true, "abs": true, "bzgl": true, "bzw": true, "bspw": true,
	"ca": true, "evtl": true, "ggf": true, "inkl": true, "kap": true,
	"nr": true, "sog": true, "usw": true, "vgl": true, "zzgl": true,
}

// A pair is an opaque span that opens and closes on fixed text.
type pair struct{ opener, closer string }

// pairs are tried in order, so a longer opener goes before a shorter one it
// starts with.
var pairs = []pair{
	{"+++", "+++"},
	{"++", "++"},
	{"``", "``"},
	{"<<", ">>"},
	{"((", "))"},
	{"[[", "]]"},
}

// escapable holds the chars whose opener a backslash in front of it cancels.
const escapable = "`+[<("

// spanEnd returns where the opaque span opening at i ends, if one opens there.
func spanEnd(src []byte, i int) (int, bool) {
	rest := src[i:]
	if rest[0] == '\\' && len(rest) > 1 && strings.IndexByte(escapable, rest[1]) >= 0 {
		return i + 2, true
	}
	for _, p := range pairs {
		if bytes.HasPrefix(rest, []byte(p.opener)) {
			return closeAt(src, i+len(p.opener), p.closer)
		}
	}
	switch {
	case rest[0] == '+' || rest[0] == '`':
		return constrained(src, i)
	case rest[0] == '[' && i > 0 && !isSpace(src[i-1]):
		return bracket(src, i)
	default:
		return 0, false
	}
}

// closeAt returns the end of the first closer from i on.
func closeAt(src []byte, i int, closer string) (int, bool) {
	n := bytes.Index(src[i:], []byte(closer))
	if n < 0 {
		return 0, false
	}
	return i + n + len(closer), true
}

// constrained returns the end of a single + or backtick span at i. As in
// Asciidoctor, the marks stand at the edges of words and hug the text between
// them, which keeps a hard line break, a + between words and the + of x+y out.
func constrained(src []byte, i int) (int, bool) {
	mark := src[i]
	before, _ := utf8.DecodeLastRune(src[:i])
	if isWordChar(before) || i+1 >= len(src) || isSpace(src[i+1]) {
		return 0, false
	}
	for j := i + 2; j < len(src); j++ {
		after, _ := utf8.DecodeRune(src[j+1:])
		if src[j] == mark && !isSpace(src[j-1]) && !isWordChar(after) {
			return j + 1, true
		}
	}
	return 0, false
}

// bracket returns the end of the attribute list a macro opens at i, counting
// the brackets nested in it.
func bracket(src []byte, i int) (int, bool) {
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j + 1, true
			}
		}
	}
	return 0, false
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

func isWordChar(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }
