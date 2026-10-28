// Package tree is the block tree the parser builds and the printer emits.
//
// The tree partitions the source: every byte belongs to exactly one node, and
// emitting the tree in order reproduces the input byte for byte. That is what
// makes a formatter with no rule enabled the identity function, and it holds
// without asking Asciidoctor anything.
package tree

import (
	"bytes"
	"fmt"
)

// Span is a byte range in the source.
type Span struct {
	Start, End int
}

// Empty reports whether the span covers no bytes.
func (s Span) Empty() bool { return s.Start == s.End }

// Gap is the blank lines in front of a node. Most of them only space the
// document out, and the printer writes them as it likes. Some carry meaning,
// and the parser says so in Stage.
type Gap struct {
	Span Span
	// Stage is how far a rule may change the blank lines.
	Stage Stage
	// BesideDirective marks a gap next to an include, a conditional or the
	// front matter. Only Asciidoctor knows what those bring in, so the blocks
	// on both sides may turn out to be one. Such a gap is always exact, and the
	// rules that rewrite a fence or a list marker leave the blocks beside it
	// alone.
	BesideDirective bool
}

// Stage is how far a rule may change the blank lines of a gap.
type Stage int

const (
	// Free leaves the blank lines to the printer.
	Free Stage = iota
	// KeepCount keeps whether there are blank lines: none stays none, one or
	// more stay at least one.
	KeepCount
	// Exact keeps the gap as it stands: the number of blank lines carries
	// meaning, or a line in it is not blank, such as a + read as one.
	Exact
)

// Delimiter is the pair of lines fencing a delimited block: Char, then Fill
// until the line is Width long. Fill is Char again, except in a table, where
// one of | , : ! opens a run of = signs.
//
// A block closes only on a line equal to the one that opened it, which is what
// makes Width part of its identity. Extensible marks a body an include or a
// conditional may extend, whose lines the parser therefore does not see in
// full.
type Delimiter struct {
	Char       byte
	Fill       byte
	Width      int
	Open       Span
	Close      Span
	Extensible bool
}

// Closed reports whether the block ended on a matching delimiter rather than at
// the end of the source.
func (d Delimiter) Closed() bool { return !d.Close.Empty() }

// Line returns the fence written width chars wide.
func (d Delimiter) Line(width int) []byte {
	line := bytes.Repeat([]byte{d.Fill}, width)
	line[0] = d.Char
	return line
}

// MetaKind tells apart the metadata lines rules address individually.
type MetaKind int

// The metadata line shapes that bind to the block below them.
const (
	MetaAttributes MetaKind = iota // [source,go]
	MetaTitle                      // .Title
	MetaAnchor                     // [[id]]
	MetaComment                    // // comment

	// MetaCommentBlock and MetaAttrEntry are blocks of their own until metadata
	// lines stand above them. Asciidoctor reads them as metadata there and keeps
	// the lines above bound to the block that follows, so the parser does too.
	MetaCommentBlock // //// to ////
	MetaAttrEntry    // :name: value, with the lines that continue it

	// MetaDirective also freezes the block, whose real content the parser
	// cannot know once Asciidoctor has resolved the directive.
	MetaDirective
)

// Meta is a metadata line bound to the node below it. The binding survives
// blank lines, which is why it carries its own gap.
type Meta struct {
	Kind  MetaKind
	Gap   Gap
	Lines Span
}

// Node is one piece of the document. The parser produces the types below and
// no others.
type Node interface {
	Base() *NodeBase
	node()
}

// NodeBase is what every node has.
//
// Frozen marks a node whose lines have to stay as they are: the parser could
// not determine their structure, or one of them changes meaning when moved. No
// rule may add, remove, join or split lines there; line-local rewrites such as
// trailing whitespace removal stay safe.
type NodeBase struct {
	Gap    Gap
	Meta   []Meta
	Lines  Span
	Frozen bool
}

// Base implements Node.
func (b *NodeBase) Base() *NodeBase { return b }

// Extent reports every byte the node owns, its gap and metadata included.
func (b *NodeBase) Extent() Span {
	if len(b.Meta) > 0 {
		return Span{b.Meta[0].Gap.Span.Start, b.Lines.End}
	}
	return Span{b.Gap.Span.Start, b.Lines.End}
}

func (b *NodeBase) node() {}

// Header is the document header: the level 0 title, in one-line or two-line
// form, with the author, revision and attribute lines that follow it without a
// blank line.
//
// TitleLines covers the title alone, so a rule that rewrites it leaves what
// follows as it is. For a title split by a conditional, it holds the directive
// lines and the underline as well. TwoLine is the title written over two lines,
// which turns on compat-mode and therefore may not be collapsed.
type Header struct {
	NodeBase

	Title      Span
	TitleLines Span
	TwoLine    bool
}

// Heading is a one-line section title. Marker is = or #, since Asciidoctor
// reads a Markdown heading as a section title too.
type Heading struct {
	NodeBase

	Marker byte
	Level  int
	Title  Span
}

// Setext is a two-line section title. Asciidoctor reads the pair before it
// reads the underline as a delimiter, and so does the parser.
type Setext struct {
	NodeBase

	Level int
	Title Span
}

// Paragraph holds prose.
type Paragraph struct{ NodeBase }

// Admonition is a paragraph whose first line opens with a label such as NOTE:,
// which Asciidoctor renders as an admonition block rather than as prose.
type Admonition struct{ NodeBase }

// Literal holds verbatim lines without a delimiter: an indented paragraph, one
// an attribute line turned into code, or prose whose line breaks carry meaning,
// because hardbreaks are on, a missing attribute drops its line, or it sets
// substitutions of its own.
type Literal struct{ NodeBase }

// Verbatim is a delimited block whose content must stay byte-identical:
// listing, literal, passthrough and comment blocks, fenced code, and a block
// with a style only an extension knows.
type Verbatim struct {
	NodeBase

	Delim Delimiter
}

// Container is a delimited block holding further nodes: example, sidebar,
// quote and open blocks.
type Container struct {
	NodeBase

	Delim    Delimiter
	Children []Node
	Tail     Gap
}

// Table stays opaque until a rule needs its cells.
type Table struct {
	NodeBase

	Delim Delimiter
}

// List is a run of items sharing one marker. A different marker nests, which is
// how AsciiDoc spells nesting.
//
// Open holds the markers of the lists this one stands in, innermost last, and
// Section reports whether it stands where a section title can be read at all.
//
// Extensible is Delimiter.Extensible for a list, which has no fence to carry
// it: a directive inside an item or right under it may still bring items in,
// because nothing closes a list.
//
// Description reports whether the list is a description list, whose items are
// terms.
type List struct {
	NodeBase

	Marker      string
	Open        []string
	Section     bool
	Extensible  bool
	Description bool
	Items       []*ListItem
}

// ListItem is one entry. Principal holds the lines the item owns before its
// children, marker and comments included; Children are the blocks attached to
// it.
//
// Text is the item's text as the parser reads it, its trailing whitespace off:
// behind the marker, or, for a term with nothing there, on the lines under it.
// Empty, it marks a term standing without text.
type ListItem struct {
	NodeBase

	Marker    Span
	Principal Span
	Text      Span
	Children  []Node
}

// Continuation is a lone + line. The blank lines before it select the list
// level the following block attaches to, so its gap is exact.
type Continuation struct{ NodeBase }

// Attribute is an attribute entry (:name: value) with the lines that continue
// its value.
type Attribute struct{ NodeBase }

// Directive is an include, ifdef, ifndef, ifeval or endif line. What it brings
// in decides the structure around it, so the parser freezes rather than
// resolves.
type Directive struct{ NodeBase }

// FrontMatter is the YAML block some static site generators put first. It is
// not AsciiDoc and passes through untouched.
type FrontMatter struct{ NodeBase }

// Opaque is a block the parser delimits but does not model: a block macro, a
// thematic or page break, a Markdown quote, a quoted paragraph with its
// attribution line, a paragraph an extension may read, or metadata that never
// found its block. It is neither prose nor verbatim content, so no rule
// reflows it, and it passes through unchanged.
//
// A construct leaves this type when a rule needs it told apart from the rest.
type Opaque struct{ NodeBase }

// Finding is what the parser could not decide, and where.
type Finding struct {
	Line    int
	Message string
}

func (f Finding) String() string { return fmt.Sprintf("line %d: %s", f.Line, f.Message) }

// LineEnding is what ends a line. block.Terminator answers the same question
// for one line, and may answer that it ends without one; a document always has
// an answer, because a rule writing a line of its own needs one to write.
type LineEnding string

// The two forms a source may end a line with.
const (
	LF   LineEnding = "\n"
	CRLF LineEnding = "\r\n"
)

// Document is a parsed source file. The parser refuses a source that mixes
// the two line endings, so LineEnding holds for the whole of it.
type Document struct {
	Src        []byte
	BOM        Span
	LineEnding LineEnding
	Nodes      []Node
	Tail       Gap
	Findings   []Finding
}
