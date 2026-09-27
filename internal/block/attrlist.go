// What a block attribute line [...] says: the style and the options it sets.

package block

import (
	"bytes"
	"slices"
	"strings"
)

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

// strictVerbatimStyles are the styles Asciidoctor checks before it looks at the
// line below them, its VERBATIM_STYLES: a marker, an indent or a delimiter
// there is content, and the paragraph runs to the next blank line.
var strictVerbatimStyles = map[string]bool{"literal": true, "listing": true, "source": true, "verse": true}

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

// extensionStyle reports whether a style is one Asciidoctor knows only from an
// extension, which reads the block in its own way. Without one loaded the
// style is dropped, and the formatter cannot see which are.
func extensionStyle(style string) bool {
	return style != "" && style != "normal" && !paragraphStyles[style] && !verbatimStyles[style]
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
	var opts []string
	for i, attr := range attrList(s) {
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

// substitutionsAttr sets which substitutions the text of a block goes through.
// A paragraph that sets its own may pass raw HTML through, where a line break
// shows, so its line breaks are content.
const substitutionsAttr = "subs"

// hasAttr reports whether an attribute line above the block sets the named
// attribute.
func hasAttr(src []byte, meta []Meta, attr string) bool {
	for _, m := range meta {
		if m.Kind != MetaAttributes {
			continue
		}
		for _, a := range attrList(src[m.Lines.Start:m.Lines.End]) {
			if name, _, named := cutAttr(a); named && name == attr {
				return true
			}
		}
	}
	return false
}

// attrList returns the attributes of an attribute line, or none where s is no
// attribute line.
func attrList(s []byte) [][]byte {
	s = bytes.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil
	}
	return splitAttrList(s[1 : len(s)-1])
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
