// Package block cuts AsciiDoc source into the block tree the printer emits.
//
// The tree partitions the source: every byte belongs to exactly one node, and
// emitting the tree in order reproduces the input byte for byte. That is what
// makes a formatter with no rule enabled the identity function, and it holds
// without asking Asciidoctor anything.
//
// The scanner never resolves an include or a conditional, and never guesses.
// Content it cannot know it freezes; structure it cannot trust it reports.
package block

import "fmt"

// Span is a byte range in the source.
type Span struct {
	Start, End int
}

// Empty reports whether the span covers no bytes.
func (s Span) Empty() bool { return s.Start == s.End }

// Gap is the blank lines in front of a node.
//
// Frozen marks the ones that carry meaning: before a list continuation they
// select the level the following block attaches to, and around a directive
// they decide whether two blocks merge when rendered.
type Gap struct {
	Span   Span
	Frozen bool
}

// Delimiter is the pair of lines fencing a delimited block. Blocks pair by char
// and exact width, so shortening one is only safe when its body holds no run of
// the same char.
type Delimiter struct {
	Char  byte
	Width int
	Open  Span
	Close Span
}

// Closed reports whether the block ended on a matching delimiter rather than at
// the end of the source.
func (d Delimiter) Closed() bool { return !d.Close.Empty() }

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
	// the lines above bound to the block that follows, so the scanner does too.
	MetaCommentBlock // //// to ////
	MetaAttrEntry    // :name: value, with the lines that continue it

	// MetaDirective also freezes the block, whose real content the scanner
	// cannot know once Asciidoctor has resolved the directive.
	MetaDirective
)

// Meta is a metadata line bound to the node below it. The binding survives
// blank lines, which is why it carries its own gap.
//
// Open marks an attribute entry whose value ends only at the blank line below
// it, because its last line still carries the wrap marker. That blank line is
// syntax: without it the value reads on and swallows the block beneath.
type Meta struct {
	Kind  MetaKind
	Gap   Gap
	Lines Span
	Open  bool
}

// Node is one piece of the document. The scanner produces the types below and
// no others.
type Node interface {
	// Extent reports every byte the node owns, its gap and metadata included.
	// Emitting it verbatim is what a print function does until a rule claims
	// the node.
	Extent() Span
	Gap() Gap
	Meta() []Meta
	Lines() Span
	Frozen() bool
	node()
}

// base is what every node has.
//
// Frozen marks a node whose lines have to stay as they are: the scanner could
// not determine their structure, or one of them changes meaning when moved. No
// rule may add, remove, join or split lines there; line-local rewrites such as
// trailing whitespace removal stay safe.
type base struct {
	gap    Gap
	meta   []Meta
	lines  Span
	frozen bool
}

func (b *base) node() {}

// Extent implements Node.
func (b *base) Extent() Span {
	if len(b.meta) > 0 {
		return Span{b.meta[0].Gap.Span.Start, b.lines.End}
	}
	return Span{b.gap.Span.Start, b.lines.End}
}

func (b *base) Gap() Gap     { return b.gap }
func (b *base) Meta() []Meta { return b.meta }
func (b *base) Lines() Span  { return b.lines }
func (b *base) Frozen() bool { return b.frozen }

// Header is the document header: the level 0 title, in one-line or two-line
// form, with the author, revision and attribute lines that follow it without a
// blank line.
//
// TitleLines covers the title alone, so a rule that rewrites it leaves what
// follows as it is. TwoLine is the title written over two lines, which turns on
// compat-mode and therefore may not be collapsed.
type Header struct {
	base
	Title      Span
	TitleLines Span
	TwoLine    bool
}

// Heading is a one-line section title. Marker is = or #, since Asciidoctor
// reads a Markdown heading as a section title too.
type Heading struct {
	base
	Marker byte
	Level  int
	Title  Span
}

// Setext is a two-line section title. Asciidoctor reads the pair before it
// reads the underline as a delimiter, and so does the scanner.
type Setext struct {
	base
	Level int
	Title Span
}

// Paragraph holds prose, the only content a sentence rule may reflow.
type Paragraph struct{ base }

// Literal holds verbatim lines without a delimiter: an indented paragraph, one
// an attribute line turned into code, or prose whose line breaks Asciidoctor
// renders, because hardbreaks are on or a missing attribute drops its line.
type Literal struct{ base }

// Verbatim is a delimited block whose content must stay byte-identical:
// listing, literal, passthrough and comment blocks, and fenced code.
type Verbatim struct {
	base
	Delim Delimiter
}

// Container is a delimited block holding further nodes: example, sidebar,
// quote and open blocks.
type Container struct {
	base
	Delim    Delimiter
	Children []Node
	Tail     Gap
}

// Table stays opaque until a rule needs its cells.
type Table struct {
	base
	Delim Delimiter
}

// List is a run of items sharing one marker. A different marker nests, which is
// how AsciiDoc spells nesting.
type List struct {
	base
	Marker string
	Items  []*ListItem
}

// ListItem is one entry. Principal is its own text, Children are the blocks
// attached to it.
type ListItem struct {
	base
	Marker    Span
	Principal Span
	Children  []Node
}

// Continuation is a lone + line. The blank lines before it select the list
// level the following block attaches to, so its gap is frozen.
type Continuation struct{ base }

// Attribute is an attribute entry (:name: value) with the lines that continue
// its value.
type Attribute struct{ base }

// Directive is an include, ifdef, ifndef, ifeval or endif line. What it brings
// in decides the structure around it, so the scanner freezes rather than
// resolves.
type Directive struct{ base }

// FrontMatter is the YAML block some static site generators put first. It is
// not AsciiDoc and passes through untouched.
type FrontMatter struct{ base }

// Opaque is a block the scanner delimits but does not model: a block macro, a
// thematic or page break, a Markdown quote, a quoted paragraph with its
// attribution line, or metadata that never found its block. It is neither
// prose nor verbatim content, so no rule reflows it, and it passes through
// unchanged.
//
// A construct leaves this type when a rule needs it told apart from the rest.
type Opaque struct{ base }

// Finding is what the scanner could not decide, and where.
type Finding struct {
	Line    int
	Message string
}

func (f Finding) Error() string { return fmt.Sprintf("line %d: %s", f.Line, f.Message) }

// LineEnding is what ends a line. Terminator answers the same question for one
// line, and may answer that it ends without one; a document always has an
// answer, because a rule writing a line of its own needs one to write.
type LineEnding string

// The two forms a source may end a line with.
const (
	LF   LineEnding = "\n"
	CRLF LineEnding = "\r\n"
)

// Document is a scanned source file. The scanner refuses a source that mixes
// the two line endings, so LineEnding holds for the whole of it.
type Document struct {
	Src        []byte
	BOM        Span
	LineEnding LineEnding
	Nodes      []Node
	Tail       Gap
	Findings   []Finding
}
